// Copyright 2011 Percy Wegmann. All rights reserved.
// Use of this source code is governed by the BSD license found in LICENSE.

/*
   Package gflow provides a mechanism for defining and processing
   event-driven flows.

   Such a flow accepts events one at a time and advances the flow depending on
   whether or not the event meets the required conditions.

   Let a, b, c ... equal a set of conditions for advancing a flow
   Let A, B, C ... equal a set of events that meet the relevant conditions

   A flow can be composed of these conditions using the operators

   THEN OR AND

   For example, given the flow defined as:

   a.THEN(a).THEN(b)
    .OR(c.AND(d))

   Any of the following series of events will complete the flow:

   A -> A -> B
   C -> D
   D -> C

   Because each event can only advance the flow by 1 step, the following would not complete the flow:

   A -> B (the A is NOT double-counted)

   Given an action "action", a flow can be configured to fire that action
   upon completion using the DO operator.

   For example, given:

   a.THEN(b).DO(action)

   action will fire immediately after the sequence:

   A -> B

   Whether or not a given transition is allowed is encapsulated by a Test,
   which is just a function like this:

   var a gflow.Test = func(data gflow.ProcessData) bool {
       return // Test data and return a bool
   }

   Flows are defined by composing Tests using the aforementioned operators:

   flowDefinition := a.THEN(a).THEN(b).OR(c.AND(d))

   Event data is passed around flows in the form of a ProcessData,
   which is just a map[string]string:

   eventA := gflow.ProcessData{key: val, anotherKey: anotherVal}

   To start a flow, simply call the function Start and pass it a ProcessData
   for the first event:

   flow = flowDefinition.Start(eventA)

   Start returns a state representing the result of evaluating the event.
   This will be either the original state if we did not advance 
   or a new state if we did advance. 

   IMPORTANT: once constructed, a flow is immutable and thread-safe.
   
   To continue advancing through a flow starting at the last state,
   use the method Advance:

   flow = flow.Advance(eventB)

   Just liks Start, Advance returns a state.

   To tell whether or not a flow is finished, use the method Finished:

   isFinished = flow.Finished()

   Once a flow is finished, it is legal to keep sending events to it,
   but this will have no effect.
   
   Since the flows are stateless, it is up to the client code to track
   the current state of the flow by hanging on to the return value
   from each call to Start() and Advance().
*/
package gflow

// Test is any function that tests against a given ProcessData and returns
// a bool indicating whether or not the flow is allowed to transition.
type Test func(data ProcessData) bool

// Action is any function that executes at the end of a flow.
type Action func(data ProcessData)

// ProcessData is a map of data that is passed through the process flow to Tests and Actions.
type ProcessData map[string]string

// flowState represents a state in the flow, including inbound and outbound transitions
// and, if applicable, the Action executed when this flowState is reached.
type flowState struct {
	in          []*transition
	out         []*transition
	andedStates []*flowState
	action      Action
}

// stateSource is any object that can be converted into a flowState.
// Test objects are stateSources, meaning that a Test can be treated
// the same as a flowState.
type stateSource interface {
	state() *flowState
}

// transition represents a transition from one flowState to another flowState
// contingent on a given Test.
type transition struct {
	test Test
	from *flowState
	to   *flowState
}

// THEN constructs a sequential flow which terminates when the from and to
// flowStates are reached in sequence. 
func (from *flowState) THEN(to stateSource) *flowState {
	toState := to.state()
	root := toState.root()
	for _, trans := range root.out {
		from.addOut(trans)
	}
	return toState
}

func (from Test) THEN(to stateSource) *flowState {
	return from.state().THEN(to)
}

/*
   OR constructs a conditional flow which terminates when either the
   state or the other state are reached.

   OR is commutative - a.OR(b) is the same as b.OR(a)
*/
func (state *flowState) OR(other stateSource) *flowState {
	otherState := other.state()
	// Create a common start node
	start := new(flowState)
	// Create a common end node
	end := new(flowState)

	root := state.root()
	otherRoot := otherState.root()

	start.addOrStates(root, otherRoot, end)
	return end
}

func (test Test) OR(other stateSource) *flowState {
	return test.state().OR(other)
}

/*
   AND constructs a flow which terminates when both
   state and other are reached.

   AND is commutative - a.AND(b) is the same as b.AND(a)
*/
func (state *flowState) AND(other stateSource) *flowState {
	otherState := other.state()
	// Create a common start node
	start := new(flowState)
	// Create a common end node
	end := new(flowState)

	andedStates := state.andedStates
	if len(andedStates) == 0 {
		andedStates = append(andedStates, state)
	}
	andedStates = append(andedStates, otherState)
	end.andedStates = andedStates

	andedRoots := make([]*flowState, len(andedStates))
	for i, state := range andedStates {
		andedRoots[i] = state.root()
	}

	start.addAndStates(andedRoots, end)

	return end
}

func (test Test) AND(other stateSource) *flowState {
	return test.state().AND(other)
}

// DO registers the given action to fire when the state is reached.
func (state *flowState) DO(action Action) *flowState {
	state.action = action
	return state
}

// Start starts a new flow from the root of the given flowState.
func (state *flowState) Start(data ProcessData) *flowState {
	root := state.root()
	return root.Advance(data)
}

func (state *flowState) Advance(data ProcessData) *flowState {
	// Go through outbound transitions and see which pass the test
	for _, tran := range state.out {
		if tran.test(data) {
			// Transition test passed, advance
			if tran.to.action != nil {
				// Execute the action
				tran.to.action(data)
			}
			// Advance to the next State
			return tran.to
		}
	}
	return state
}

// Finished indicates whether or not the flow is finished.
func (state *flowState) Finished() bool {
	return len(state.out) == 0
}

/* PRIVATE FUNCTIONS */
// state is provided to make flowState itself a StateSource.
func (state *flowState) state() *flowState {
	return state
}

// state is provided to make test behave as a StateSource.
func (test Test) state() *flowState {
	from := new(flowState)
	to := new(flowState)
	trans := &transition{test: test, from: from, to: to}
	to.addIn(trans)
	from.addOut(trans)
	return to
}

// addIn adds an inbound transition to the given state, updating the
// transition to reference the state as its "to".
func (state *flowState) addIn(trans *transition) {
	trans.to = state
	state.in = append(state.in, trans)
}

// addOut adds an outbound transition to the given state, updating the
// transition to reference the state as its "from"
func (state *flowState) addOut(trans *transition) {
	trans.from = state
	state.out = append(state.out, trans)
}

// root finds the root state of the flow, starting from the given state.
func (state *flowState) root() *flowState {
	if len(state.in) == 0 {
		return state
	}
	return state.in[0].from.root()
}

// copy makes a deep copy of the given state.  The copy is deep because
// all transitively referenced states (inbound and outbound) are copied also.
func (state *flowState) copy() *flowState {
	stateCopies := make(map[*flowState]*flowState)

	state.root().doCopy(stateCopies)

	return stateCopies[state]
}

func (state *flowState) doCopy(stateCopies map[*flowState]*flowState) *flowState {
	stateCopy := stateCopies[state]
	if stateCopy == nil {
		stateCopy = new(flowState)
		stateCopies[state] = stateCopy
	}

	for _, out := range state.out {
		newTo := out.to.doCopy(stateCopies)
		trans := &transition{test: out.test, from: stateCopy, to: newTo}
		stateCopy.addOut(trans)
		newTo.addIn(trans)
	}

	for _, andedState := range state.andedStates {
		stateCopy.andedStates = append(stateCopy.andedStates, stateCopies[andedState])
	}

	stateCopy.action = state.action
	return stateCopy
}

// addOrStates provides the functionality for recursively building a tree of states
// that model an OR condition.
func (state *flowState) addOrStates(left *flowState, right *flowState, end *flowState) {
	for _, trans := range left.out {
		atEnd := len(trans.to.out) == 0
		var next *flowState
		if atEnd {
			next = end
		} else {
			next = new(flowState)
		}
		newTrans := &transition{test: trans.test, from: state, to: next}
		state.addOut(newTrans)
		next.addIn(newTrans)
		if !atEnd {
			next.addOrStates(trans.to, right, end)
		}
	}
	for _, trans := range right.out {
		atEnd := len(trans.to.out) == 0
		var next *flowState
		if atEnd {
			next = end
		} else {
			next = new(flowState)
		}
		newTrans := &transition{test: trans.test, from: state, to: next}
		state.addOut(newTrans)
		next.addIn(newTrans)
		if !atEnd {
			next.addOrStates(left, trans.to, end)
		}
	}
}

// addAndStates provides the functionality for recursively building a tree of states
// that model an AND condition.
func (state *flowState) addAndStates(andedStates []*flowState, end *flowState) {
	atEnd := true
	totalOuts := 0
	for _, andedState := range andedStates {
		totalOuts += len(andedState.out)
	}
	for i, andedState := range andedStates {
		for _, trans := range andedState.out {
			atEnd = false
			next := new(flowState)
			newTrans := &transition{test: trans.test, from: state, to: next}
			state.addOut(newTrans)
			next.addIn(newTrans)
			var nextAndedStates []*flowState
			nextAndedStates = replace(andedStates, i, trans.to)
			next.addAndStates(nextAndedStates, end)
		}
	}
	if atEnd {
		for _, trans := range state.in {
			// Switch the transition to terminate at the end state
			end.addIn(trans)
		}
	}
}

// replace replaces the state at the given position in the given state slice
// with the given state.
func replace(states []*flowState, index int, state *flowState) []*flowState {
	var result []*flowState
	result = append(result, states[0:index]...)
	result = append(result, state)
	result = append(result, states[index+1:len(states)]...)
	return result
}

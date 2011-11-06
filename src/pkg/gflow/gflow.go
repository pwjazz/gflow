// Copyright 2011 Percy Wegmann. All rights reserved.
// Use of this source code is governed by the BSD license found in LICENSE.

/*
   Package gflow provides a mechanism for defining and processing
   event-driven flows, which we'll just call 'flows'.

   A flow accepts events one at a time and advances the flow depending on
   whether or not the event meets the required conditions.

   Let a, b, c ... equal a set of conditions for advancing a flow
   Let A, B, C ... equal a set of events that meet the relevant conditions

   flows are composed of a series of Tests that are combined using the below
   operators:
   
   THEN     a simple sequence
   OR       logical OR (commutative)
   AND      logical AND (commutative)
   
   A Test is simply a function that accepts an EventData object (just a 
   map[string][string]) and returns true or false depending on whether or not
   the flow is allowed to proceed.
   
   For example:
   
   var a gflow.Test = func(data gflow.EventData) bool {
       // Test data and return a bool
   }
   
   A flow is constructed using the aforementioned operators concluded by
   a call to the method Build().
   
   For example:
   
   flow := a.THEN(a).THEN(b)
            .OR(
                c.AND(d)).Build()
                
   The different branches of a flow can be constructed individually and
   composed into a larger flow later.  The below code creates a flow equivalent
   to the above:
   
   branch1 := a.THEN(a).THEN(b)
   branch2 := c.AND(d)
   
   flow := branch1.OR(branch2).Build()
                
   Flows may include Actions that fire when a certain state is reached.
   An Action is simply a void function that takes an EventData.
   Actions are attached to flows using the DO method.
   
   For example:
   
   action := func(data EventData) {
       fmt.Println("I reached a state!")
   }
   
   flow := branch1.OR(branch2).DO(action)
   
   In this example, when either branch1 or branch2 is completed, action will be
   called.
   
   Given the above example, any of the following series of events will complete
   the flow:

   A -> A -> B
   C -> D
   D -> C

   Because each event can only advance the flow by 1 step, the following would
   not complete the flow:

   A -> B (the A is NOT double-counted)
   
   Event data is passed around flows in the form of an EventData, which is just
   a map[string]string.
   
   For example:

   eventA := gflow.EventData{key: val, anotherKey: anotherVal}
   eventB := gflow.EventData{key: val, anotherKey: anotherVal}
   
   One advances through flows by passing an EventData to the Advance()
   method. Advance() returns a state representing the current state of the
   flow, which itself can be Advanced.
   
   For example:
   
   state := flow.Advance(eventA)
   state = state.Advance(eventB)
   
   IMPORTANT: flows are immutable and therefore are thread-safe.

   It is up to the client to maintain the current state of the flow by hanging
   on to the state returned by Advance(). To help clients manage long running
   flows, gflow provides an ID property on states and a FindByID method to
   retrieve a state from a flow by its ID.
   
   For example:
   
   state := flow.Advance(eventA)
   stateId = state.ID
   // Save ID
   // Do some other stuff
   
   // Later ...
   resumedState := flow.FindByID(stateId)
   resumedState = resumedState.Advance(eventB)
   
   IMPORTANT: if the definition of a flow changes, even if it is logically
   equivalent to the previous definition, saved IDs will no longer be reliable.
   Therefore, once you have started using a flow, you should NEVER change its
   definition until the you no longer have any such flows in flight.
   
   For example:
   
   flow1 := a.OR(b).Build()
   flow2 := b.OR(a).Build()
   
   Logically, these flows are the same, but the id's from these flows cannot be
   interchanged.  So, once you've built flow1 and are hanging on to ids from
   that flow, you should make sure to continue defining the flow using the same
   statement.
   
   To tell whether or not a flow is finished, you can use the method Finished:

   isFinished = state.Finished()
   
   Finished just means that there are no further transitions left in the flow.

   Once a flow is finished, it is legal to keep sending events to it,
   but this will have no effect.
   
   --- How it Works ---
   A gflow flow is a state machine that describes flows as a series of states
   connected by transitions.  Every flow is a directed graph that starts at a
   common root and ends at a common endpoint.
   
   Most of the methods used by gflow accept and return return a state, including
   the compositional methods THEN, OR and AND.  For convenience, tests can
   implicitly be treated as a state.
   
   state := a.THEN(b)
   
   In the above statement, the tests a and b are converted implicitly into
   states, which are then composed using the THEN method, which yields a
   new state representing the end of the current flow.
   
   state = state.OR(c.AND(d))
   
   In the above statement, we're logically OR'ing the original state with a new
   state that is itself the composite of c and d.
   
   Because everything is actually just a state, we can define individual parts
   of a flow and compose them later, in effect composing flows into larger ones:
   
   branch1 := a.THEN(a).THEN(b)
   branch2 := c.AND(d)
   large := branch1.OR(branch2)
   
   All that the Build() function actually does is to return the root (starting)
   node for the flow in which 'state' is a participating state.  Build() also
   ensures that all of the states in the flow have a unique ID assigned to them,
   which as mentioned before is just a convenience to help clients manage long-
   running flows.  Note: the variable 'flow' here actually refers to a state.
   
   flow := state.Build()
   
   With Advance, we send events to the state/flow and it gives us back the next
   state.
   
   flow = flow.Advance(...)
   
   The states themselves are stateless.  In other words, calling Advance() has
   no side effects on the states themselves, and the current position in the
   flow is fully encapsulated by the state returned from Advance().
   Consequently, it is perfectly safe to use a single flow multiple times,
   from multiple threads.  In fact, all of the methods on a state, including
   the compositional methods THEN, OR and AND, are thread-safe.  The method
   Advance() simply moves through the state tree and returns the appropriate
   state.  The methods THEN, OR and AND actually create new states, leaving
   the old state/flow untouched.
*/
package gflow

// Test is any function that tests against a given EventData and returns
// a bool indicating whether or not the flow is allowed to transition.
type Test func(data EventData) bool

// Action is any function that executes at the end of a flow.
type Action func(data EventData)

// EventData is a map of data that is passed through the process flow to Tests and Actions.
type EventData map[string]string

// flowState represents a state in the flow, including inbound and outbound transitions
// and, if applicable, the Action executed when this flowState is reached.
type flowState struct {
    ID          int
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
    newFrom := from.copy()
    toState := to.state().copy()
    for _, trans := range toState.root().out {
		newFrom.addOut(trans)
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
func (state *flowState) Build() *flowState {
	root := state.root()
	root.assignIds(0)
    return root
}

func (state *flowState) Advance(data EventData) *flowState {
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

func (state *flowState) FindByID(id int) *flowState {
    if state.ID == id {
        return state
    }
    for _, trans := range state.out {
        result := trans.to.FindByID(id)
        if result != nil {
            return result
        }
    }
    return nil
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

func (state *flowState) assignIds(startingId int) int {
    currentId := startingId + 1
    state.ID = currentId
    for _, trans := range state.out {
        currentId = trans.to.assignIds(currentId)
    }
    return currentId
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

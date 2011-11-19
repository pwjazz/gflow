// Copyright 2011 Percy Wegmann. All rights reserved.
// Use of this source code is governed by the BSD license found in LICENSE.

/*
   --------------------------------- OVERVIEW ---------------------------------

   Package gflow provides a mechanism for defining and processing
   event-driven flows, which we'll just call 'flows'. These flows are
   immutable and side-effect free, making them safe to use in multi-threaded
   environments.  Flows can be composed of other flows, making it easy to
   create relatively complex flows from smaller constituent parts.

   A flow is a directed acyclic graph of states, sharing the same start and end
   states and connected by one or more transitions from one state to the next.
   Each transition is governed by a single test, which is specified as a
   function. These tests must be mutually exclusive.

   ---------------------------- STRUCTURE OF FLOWS ----------------------------

   Let a, b, c ... equal a set of tests for advancing a flow
   Let A, B, C ... equal a set of events that pass the respective tests
   Let @ represent a state
   Let --a-->, --b-->, ... represent transitions dependent on events a, b, etc.
   Let A -> B -> C -> ... represent a sequence of events

   For example, the below is a diagram of a 2-state flow with a single test 'a'
   satisfied by a one-event sequence:

   @ --a--> @

   Satisfied by:

   A

   Multiple tests are composed into a flow using the following operators:

   THEN     a sequence of tests
   OR       logical OR (commutative)
   AND      logical AND (commutative)

   The flow a THEN a THEN b is structured as follows:

   @ --a--> @ --a--> @ --b--> @

   Satisfied by:

   A -> A -> B

   Note: each event may trigger only 1 transition at a time, thus the above
   flow is not satisfied by A -> B because A is not double counted.

   Note: events that do not trigger a transition are effectively ignored, thus
   the above flow is satisfied equally well by all three of the following:

   A -> A -> B
   A -> X -> A -> Y -> B
   A -> A -> A -> A -> B

   The flow c AND d is structured as follows:

   |--c--> @ --d-->| 
   @               @
   |--d--> @ --c-->|

   Satisfied by:

   C -> D
   D -> C

   The flow e OR f is structured as follows:

   |--e-->|
   @      @
   |--f-->|

   Satisfied by:

   E
   F

   Flows can be composed by the same operators as tests, for example:

   a THEN a THEN b OR (c AND d)

   Satisfied by:

   A -> A -> B
   C -> D
   D -> C

   ------------------------------- CODING ------------------------------------

   // This example uses the flow a THEN a THEN b OR (c AND d)

   // Define tests.  Tests are functions that accept an EventData and
   // return a bool.  EventData is the empty interface, meaning it can be
   // any type of value.

   // In our example, we're just using strings as our EventData.

   var a gflow.Test = func(data gflow.EventData) bool {
       return "A" == data.(string)
   }

   var b gflow.Test = func(data gflow.EventData) bool {
       return "B" == data.(string)
   }

   // ... and so on ...

   // The flow is defined using the methods THEN OR and AND, which return
   // state objects.

   flow := a.THEN(a).THEN(b).OR(c.AND(d))

   // Each of these compositional methods accepts two tests and returns a new
   // state.  Since states can be composed using these same methods, it is
   // possible to break apart the definition of flows however is convenient.

   subFlow1 := a.THEN(a).THEN(b)
   subFlow2 := c.AND(d)
   redefined := subFlow1.OR(subFlow2)
   // the above redefined is equivalent to the original flow

   // As mentioned previously, events can be anything, and in our case are just
   // strings.

   eventA := "A"
   eventB := "B"

   // To use a flow, call the Build() method and then Advance().
   // Build() basically just returns the root state of the flow so that
   // you start from the beginning.  It also assigns ids to states, which is
   // discussed further below.

   state := flow.Build().Advance(eventA)

   // Advance() returns the next state based on whatever transition fired (or
   // did not fire).  Continue advancing by calling Advance() on the returned
   // states.

   state = state.Advance(eventB)
   state = state.Advance(eventC)
   // ... and so on ...

   // Above we assigned the result from Advance() to a variable named "state".
   // In reality, state and flow are interchangeable - it's all just states.
   // A flow can be referenced by any state in that flow, since they all
   // share the same root.

   state = state.Build().Advance(eventA).Advance(eventB)

   // You can check whether a flow is finished by using the method Finished.

   isFinished := state.Finished()

   // Finished just means that there are no further transitions left.

   // Once a flow is finished, it is legal to keep sending events to it,
   // but this will have no effect.

   // IMPORTANT - gflow states are immutable and their methods are free of
   // side-effects, making them safe to use in a multi-threaded environment.

   // For example, the below line creates a new flow from subFlow1 that
   // requires the additional step f.

   subFlow1 = subFlow1.THEN(f)

   // Sub flow 1 is now @ --a--> @ -->a--> @ --b--> @ --f--> @

   // Note how we assigned the result of THEN() to a variable.  The original
   // state referred to by subFlow1 before the assignment has not changed.  So,
   // any in-flight processes based on that flow can continue unhampered.

   // The same property of thread-safety holds true for Advance().

   state1 := flow.Advance(eventA)
   state2 := flow.Advance(eventA)

   // The above two states are completely independent of each other.

   // The stateless nature of gflow means that the responsibility for managing
   // the current state of flows is entirely the client program's.

   // To help clients manage long running flows, gflow provides an ID property
   // on states and a FindByID method to retrieve a state from a flow by its ID.

   state = flow.Advance(eventA)
   stateId = state.ID
   // Save ID
   // Do some other stuff
   // Later ...
   resumedState := flow.FindByID(stateId)
   resumedState = resumedState.Advance(eventB)

   // IMPORTANT: if the definition of a flow changes, even if it is logically
   // equivalent to the previous definition, saved IDs will no longer be
   // reliable. Therefore, once you have started using a flow, you should
   // NEVER change its definition unless and until you no longer have any such
   // flows in flight.  For example:

   flow1 := a.AND(b).OR(c.AND(d)).Build()
   flow2 := c.AND(d).OR(a.AND(b)).Build()

   s1 := flow1.Advance(eventA)
   id := s1.ID
   s2 := flow2.FindByID(id)

   // Logically, these flows are the same, but the id's from these flows cannot
   // be interchanged.  In this case, s1 and s2 are NOT equivalent!
*/
package gflow

// Test is any function that tests against a given EventData and returns
// a bool indicating whether or not the flow is allowed to transition.
type Test func(data EventData) bool

// Action is any function that executes at the end of a flow.
type Action func(data EventData)

// EventData any object
type EventData interface{}

// State represents a state in the flow, including inbound and outbound
// transitions and, if applicable, the Action executed when this State is
// reached.
type State struct {
	ID          int
	in          []*transition
	out         []*transition
	andedStates []*State
	action      Action
}

// stateSource is any object that can be converted into a State.
// Test objects are stateSources, meaning that a Test can be treated
// the same as a State.
type stateSource interface {
	state() *State
}

// transition represents a transition from one State to another State
// contingent on a given Test.
type transition struct {
	test Test
	from *State
	to   *State
}

// THEN constructs a sequential flow which terminates when the from and to
// States are reached in sequence. 
func (from *State) THEN(to stateSource) *State {
	newFrom := from.copy()
	toState := to.state().copy()
	for _, trans := range toState.root().out {
		newFrom.addOut(trans)
	}
	return toState
}

func (from Test) THEN(to stateSource) *State {
	return from.state().THEN(to)
}

/*
   OR constructs a conditional flow which terminates when either the
   state or the other state are reached.

   OR is commutative - a.OR(b) is the same as b.OR(a)
*/
func (state *State) OR(other stateSource) *State {
	otherState := other.state()
	// Create a common start node
	start := new(State)
	// Create a common end node
	end := new(State)

	root := state.root()
	otherRoot := otherState.root()

	start.addOrStates(root, otherRoot, end)
	return end
}

func (test Test) OR(other stateSource) *State {
	return test.state().OR(other)
}

/*
   AND constructs a flow which terminates when both
   state and other are reached.

   AND is commutative - a.AND(b) is the same as b.AND(a)
*/
func (state *State) AND(other stateSource) *State {
	otherState := other.state()
	// Create a common start node
	start := new(State)
	// Create a common end node
	end := new(State)

	andedStates := state.andedStates
	if len(andedStates) == 0 {
		andedStates = append(andedStates, state)
	}
	andedStates = append(andedStates, otherState)
	end.andedStates = andedStates

	andedRoots := make([]*State, len(andedStates))
	for i, state := range andedStates {
		andedRoots[i] = state.root()
	}

	start.addAndStates(andedRoots, end)

	return end
}

func (test Test) AND(other stateSource) *State {
	return test.state().AND(other)
}

// DO registers the given action to fire when the state is reached.
func (state *State) DO(action Action) *State {
	state.action = action
	return state
}

// Start starts a new flow from the root of the given State.
func (state *State) Build() *State {
	root := state.root()
	root.assignIds(0)
	return root
}

func (state *State) Advance(data EventData) *State {
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

func (state *State) FindByID(id int) *State {
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
func (state *State) Finished() bool {
	return len(state.out) == 0
}

/* PRIVATE FUNCTIONS */
// state is provided to make State itself a StateSource.
func (state *State) state() *State {
	return state
}

// state is provided to make test behave as a StateSource.
func (test Test) state() *State {
	from := new(State)
	to := new(State)
	trans := &transition{test: test, from: from, to: to}
	to.addIn(trans)
	from.addOut(trans)
	return to
}

// addIn adds an inbound transition to the given state, updating the
// transition to reference the state as its "to".
func (state *State) addIn(trans *transition) {
	trans.to = state
	state.in = append(state.in, trans)
}

// addOut adds an outbound transition to the given state, updating the
// transition to reference the state as its "from"
func (state *State) addOut(trans *transition) {
	trans.from = state
	state.out = append(state.out, trans)
}

// hasTest checks whether any of the state's outbound transitions use the
// specified test
func (state *State) hasTest(test Test) bool {
	return state.transitionWithTest(test) != nil
}

func (state *State) transitionWithTest(test Test) *transition {
	for _, trans := range state.out {
		if trans.test == test {
			return trans
		}
	}
	return nil
}

// root finds the root state of the flow, starting from the given state.
func (state *State) root() *State {
	if len(state.in) == 0 {
		return state
	}
	return state.in[0].from.root()
}

// copy makes a deep copy of the given state.  The copy is deep because
// all transitively referenced states (inbound and outbound) are copied also.
func (state *State) copy() *State {
	stateCopies := make(map[*State]*State)

	state.root().doCopy(stateCopies)

	return stateCopies[state]
}

func (state *State) PubCopy() *State {
	return state.copy()
}

func (state *State) doCopy(stateCopies map[*State]*State) *State {
	stateCopy := stateCopies[state]
	if stateCopy != nil {
		return stateCopy
	}

	stateCopy = new(State)
	stateCopies[state] = stateCopy

	for _, out := range state.out {
		newTo := out.to.doCopy(stateCopies)
		trans := &transition{test: out.test, from: stateCopy, to: newTo}
		stateCopy.addOut(trans)
		newTo.addIn(trans)
	}

	for _, andedState := range state.andedStates {
		stateCopy.andedStates = append(stateCopy.andedStates, andedState.doCopy(stateCopies))
	}

	stateCopy.action = state.action
	return stateCopy
}

func (state *State) countChildren() int {
	count := len(state.out)
	for _, trans := range state.out {
		count += trans.to.countChildren()
	}
	return count
}

// addOrStates provides the functionality for recursively building a tree of
// states that model an OR condition.
func (state *State) addOrStates(left *State, right *State, end *State) {
	for _, trans := range left.out {
		atEnd := len(trans.to.out) == 0
		var next *State
		var nextLeft = trans.to
		var nextRight = right

		if right.hasTest(trans.test) {
			// The right branch has a transition with this same test.
			// Merge them by creating a new template state that combines
			// the outbound transitions from both left and right.
			rightTrans := right.transitionWithTest(trans.test)
			if len(rightTrans.to.out) == 0 {
				atEnd = true
			} else {
				nextRight = rightTrans.to
			}
		}

		if atEnd {
			next = end
		} else {
			next = new(State)
		}

		newTrans := &transition{test: trans.test, from: state, to: next}
		state.addOut(newTrans)
		next.addIn(newTrans)
		if !atEnd {
			next.addOrStates(nextLeft, nextRight, end)
		}
	}
	for _, trans := range right.out {
		if left.hasTest(trans.test) {
			// This would have already been handled in the left branch.  Skip it.
			continue
		}
		atEnd := len(trans.to.out) == 0
		var next *State
		if atEnd {
			next = end
		} else {
			next = new(State)
		}
		newTrans := &transition{test: trans.test, from: state, to: next}
		state.addOut(newTrans)
		next.addIn(newTrans)
		if !atEnd {
			next.addOrStates(left, trans.to, end)
		}
	}
}

// addAndStates provides the functionality for recursively building a tree of
// states that model an AND condition.
func (state *State) addAndStates(andedStates []*State, end *State) {
	atEnd := true
	totalOuts := 0
	for _, andedState := range andedStates {
		totalOuts += len(andedState.out)
	}
	for i, andedState := range andedStates {
		for _, trans := range andedState.out {
			atEnd = false
			next := new(State)
			newTrans := &transition{test: trans.test, from: state, to: next}
			state.addOut(newTrans)
			next.addIn(newTrans)
			var nextAndedStates []*State
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

func (state *State) assignIds(startingId int) int {
	currentId := startingId + 1
	state.ID = currentId
	for _, trans := range state.out {
		currentId = trans.to.assignIds(currentId)
	}
	return currentId
}

// replace replaces the state at the given position in the given state slice
// with the given state.
func replace(states []*State, index int, state *State) []*State {
	var result []*State
	result = append(result, states[0:index]...)
	result = append(result, state)
	result = append(result, states[index+1:len(states)]...)
	return result
}

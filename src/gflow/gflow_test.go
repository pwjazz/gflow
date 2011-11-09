package gflow

import (
	"fmt"
	"testing"
)

func makeTest(val string) Test {
	return func(data EventData) bool {
		fmt.Println(val+"?", data)
		return data.(string) == val
	}
}

var a Test = makeTest(A)
var b Test = makeTest(B)
var c Test = makeTest(C)
var d Test = makeTest(D)

var A string = "A"
var B string = "B"
var C string = "C"
var D string = "D"
var E string = "E"
var F string = "F"

type flowTest struct {
	label string
	flow  *State
	steps []string
}

var tests []flowTest = []flowTest{
	flowTest{"a THEN b THEN c THEN d",
		a.THEN(b).THEN(c.THEN(d)),
		[]string{A, B, C, D}},

	flowTest{"a OR b",
		a.OR(b),
		[]string{A}},

	flowTest{"a OR b",
		a.OR(b),
		[]string{C, B}},

	flowTest{"a THEN b THEN c THEN d",
		a.THEN(b).THEN(c).THEN(d),
		[]string{A, B, C, D}},

	flowTest{"a.THEN(b).THEN(c.THEN(d))",
		a.THEN(b).THEN(c.THEN(d)),
		[]string{A, B, C, D}},

	flowTest{"a.THEN(b).OR(c.THEN(d))",
		a.THEN(b).OR(c.THEN(d)), []string{A, C, D}},

	flowTest{"a.AND(b)",
		a.AND(b), []string{B, A, F, C}},

	flowTest{"a.AND(b).AND(c)",
		a.AND(b).AND(c),
		[]string{B, A, F, C}},

	flowTest{"a.AND(b.THEN(c))",
		a.AND(b.THEN(c)),
		[]string{A, B, C}},

	flowTest{"b.THEN(c).AND(a)",
		b.THEN(c).AND(a),
		[]string{A, B, C}},

	flowTest{"b.THEN(c).AND(a)",
		b.THEN(c).AND(a),
		[]string{B, A, C}},

	flowTest{"a.THEN(b).AND(c.OR(d))",
		a.THEN(b).THEN(d).AND(c.OR(d)),
		[]string{A, E, C, D, B, D}},

	flowTest{"a.AND(b).OR(c.AND(d))",
		a.AND(b).OR(c.AND(d)),
		[]string{A, C, B}},

	flowTest{"a.AND(b).OR(c.AND(d))",
		a.AND(b).OR(c.AND(d)),
		[]string{A, D, C}},

	flowTest{"a.AND(b).OR(a.AND(c))",
		a.AND(b).OR(a.AND(c)),
		[]string{A, D, C}},

	flowTest{"a.AND(b).OR(a.AND(c))",
		a.AND(b).OR(a.AND(c)),
		[]string{A, D, B}},

	flowTest{"a.OR(a.AND(c))",
		a.OR(a.AND(c)),
		[]string{A, C}},

	flowTest{"a.OR(a.AND(c))",
		a.OR(a.AND(c)),
		[]string{C, A}},

	flowTest{"a.OR(a.THEN(c))",
		a.OR(a.THEN(c)),
		[]string{C, A}},

	flowTest{"a.OR(c.THEN(a))",
		a.OR(c.THEN(a)),
		[]string{C, A}}}

func TestIT(t *testing.T) {

	var doTest = func(test flowTest) {
		succeeded := false

		var done = func(data EventData) {
			succeeded = true
			fmt.Println("Done!")
		}

		fmt.Println("----- TESTING ", test.label, " ------")
		flowDefinition := test.flow.DO(done).Build()
		currentId := flowDefinition.ID
		for _, key := range test.steps {
			state := flowDefinition.FindByID(currentId)
			if !state.Finished() {
				state = state.Advance(key)
				currentId = state.ID
			}
		}
		fmt.Println("")
		if !succeeded {
			t.Errorf("%s with sequence %s did not complete", test.label, test.steps)
		}
	}

	for _, test := range tests {
		doTest(test)
	}
}

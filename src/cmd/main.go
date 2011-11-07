package main

import (
	"fmt"
	"gflow"
)

func main() {
	var makeTest = func(val string) gflow.Test {
		return func(data gflow.EventData) bool {
			fmt.Println(val+"?", data)
			return data.(string) == val
		}
	}

	A := "A"
	B := "B"
	C := "C"
	D := "D"
	E := "E"
	F := "F"

	var a gflow.Test = makeTest(A)
	var b gflow.Test = makeTest(B)
	var c gflow.Test = makeTest(C)
	var d gflow.Test = makeTest(D)

	var result = func(data gflow.EventData) {
		fmt.Println("Done!")
	}

	flow := a.THEN(b).THEN(c.THEN(d)).DO(result)

	var advance = func(test string, order []string) {
		fmt.Println("----- TESTING ", test, " ------")
		flowDefinition := flow.Build()
		currentId := flowDefinition.ID
		for _, key := range order {
			state := flowDefinition.FindByID(currentId)
			if !state.Finished() {
				state = state.Advance(key)
				currentId = state.ID
			}
		}
		fmt.Println("")
	}

	advance("a THEN b THEN c THEN d", []string{A, B, C, D})

	flow = a.OR(b).DO(result)
	advance("a OR b", []string{A})

	flow = a.OR(b).DO(result)
	advance("a or b", []string{C, B})

	flow = a.THEN(b).THEN(c).THEN(d).DO(result)
	advance("a THEN b THEN c THEN d", []string{A, B, C, D})

	flow = a.THEN(b).THEN(c.THEN(d)).DO(result)
	advance("a.THEN(b).THEN(c.THEN(d))", []string{A, B, C, D})

	flow = a.THEN(b).OR(c.THEN(d)).DO(result)
	advance("a.THEN(b).OR(c.THEN(d))", []string{A, C, D})

	flow = a.AND(b).DO(result)
	advance("a.AND(b)", []string{B, A, F, C})

	flow = a.AND(b).AND(c).DO(result)
	advance("a.AND(b).AND(c)", []string{B, A, F, C})

	flow = a.AND(b.THEN(c)).DO(result)
	advance("a.AND(b.THEN(c))", []string{A, B, C})

	flow = b.THEN(c).AND(a).DO(result)
	advance("b.THEN(c).AND(a)", []string{A, B, C})

	flow = b.THEN(c).AND(a).DO(result)
	advance("b.THEN(c).AND(a)", []string{B, A, C})

	flow = a.THEN(b).THEN(d).AND(c.OR(d)).DO(result)
	advance("a.THEN(b).AND(c.OR(d))", []string{A, E, C, D, B, D})

	flow = a.AND(b).OR(c.AND(d)).DO(result)
	advance("a.AND(b).OR(c.AND(d))", []string{A, C, B})

	flow = a.AND(b).OR(c.AND(d)).DO(result)
	advance("a.AND(b).OR(c.AND(d))", []string{A, D, C})

	fmt.Println("----- FINISHED -----")
}

package main

import (
	"fmt"
	"gflow"
)

func main() {
	var a gflow.Test = func(data gflow.ProcessData) bool {
		fmt.Println("A?", data)
		return "A" == data["key"]
	}
	var b gflow.Test = func(data gflow.ProcessData) bool {
		fmt.Println("B?", data)
		return "B" == data["key"]
	}
	var c gflow.Test = func(data gflow.ProcessData) bool {
		fmt.Println("C?", data)
		return "C" == data["key"]
	}
	var d gflow.Test = func(data gflow.ProcessData) bool {
		fmt.Println("D?", data)
		return "D" == data["key"]
	}
	var result = func(data gflow.ProcessData) {
		fmt.Println("Done!")
	}

	flow := a.THEN(b).THEN(c.THEN(d)).DO(result)

	var advance = func(test string, order []string) {
		fmt.Println("----- TESTING ", test, " ------")
		flow = flow.Start(gflow.ProcessData{"key": order[0]})
		for idx, key := range order {
			if idx > 0 && !flow.Finished() {
				flow = flow.Advance(gflow.ProcessData{"key": key})
			}
		}
		fmt.Println("")
	}

	advance("a THEN b THEN c THEN d", []string{"A", "B", "C", "D"})

	flow = a.OR(b).DO(result)
	advance("a OR b", []string{"A"})

	flow = a.OR(b).DO(result)
	advance("a or b", []string{"C", "B"})

	flow = a.THEN(b).THEN(c).THEN(d).DO(result)
	advance("a THEN b THEN c THEN d", []string{"A", "B", "C", "D"})

	flow = a.THEN(b).THEN(c.THEN(d)).DO(result)
	advance("a.THEN(b).THEN(c.THEN(d))", []string{"A", "B", "C", "D"})

	flow = a.THEN(b).OR(c.THEN(d)).DO(result)
	advance("a.THEN(b).OR(c.THEN(d))", []string{"A", "C", "D"})

	flow = a.AND(b).DO(result)
	advance("a.AND(b)", []string{"B", "A", "F", "C"})

	flow = a.AND(b).AND(c).DO(result)
	advance("a.AND(b).AND(c)", []string{"B", "A", "F", "C"})

	flow = a.AND(b.THEN(c)).DO(result)
	advance("a.AND(b.THEN(c))", []string{"A", "B", "C"})

	flow = b.THEN(c).AND(a).DO(result)
	advance("b.THEN(c).AND(a)", []string{"A", "B", "C"})

	flow = b.THEN(c).AND(a).DO(result)
	advance("b.THEN(c).AND(a)", []string{"B", "A", "C"})

	flow = a.THEN(b).THEN(d).AND(c.OR(d)).DO(result)
	advance("a.THEN(b).AND(c.OR(d))", []string{"A", "E", "C", "D", "B", "D"})

	flow = a.AND(b).OR(c.AND(d)).DO(result)
	advance("a.AND(b).OR(c.AND(d))", []string{"A", "C", "B"})

	flow = a.AND(b).OR(c.AND(d)).DO(result)
	advance("a.AND(b).OR(c.AND(d))", []string{"A", "D", "C"})

	fmt.Println("----- FINISHED -----")
}

package main

import (
	"fmt"
	"log"
)

type state struct {
	count int
}

func app(ctx *Context[state]) {
	// ctx.text("now", fmt.Sprint("now: ", time.Now()))
	st := ctx.state

	ctx.text("count-text", fmt.Sprint(st.count))
	if ctx.button("Increment") {
		st.count++
	}
	if ctx.button("Decrement") {
		st.count--
	}
}

func main() {
	log.Println(run(state{0}, app).Error())
}

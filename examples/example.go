package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LullNil/orderedpool"
)

func main() {
	ctx := context.Background()
	input := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		input <- i
	}
	close(input)

	results := orderedpool.MapOrdered(ctx, input, func(ctx context.Context, x int) (string, error) {
		time.Sleep(time.Duration(x*100) * time.Millisecond)
		return fmt.Sprintf("result-%d", x), nil
	}, orderedpool.Options{
		Workers:      3,
		MaxInFlight:  5,
		EarlyStopN:   3,
		PanicAsError: true,
	})

	for res := range results {
		if res.Err != nil {
			fmt.Printf("Error: %v\n", res.Err)
		} else {
			fmt.Printf("Value: %s\n", res.Value)
		}
	}
}

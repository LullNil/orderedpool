package orderedpool

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 3)
	in <- 1
	in <- 2
	in <- 3
	close(in)

	out := MapOrdered(ctx, in, func(_ context.Context, x int) (int, error) {
		return x * 2, nil
	}, Options{Workers: 2})

	results := make([]int, 0)
	for r := range out {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		results = append(results, r.Value)
	}

	expected := []int{2, 4, 6}
	for i, v := range expected {
		if results[i] != v {
			t.Errorf("expected %v, got %v", v, results[i])
		}
	}
}

func TestEarlyStop(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		in <- i
	}
	close(in)

	out := MapOrdered(ctx, in, func(_ context.Context, x int) (int, error) {
		return x * 2, nil
	}, Options{Workers: 2, EarlyStopN: 2})

	count := 0
	for r := range out {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		count++
	}

	if count != 2 {
		t.Fatalf("expected 2 results, got %d", count)
	}
}

func TestTimeout(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 1)
	in <- 1
	close(in)

	out := MapOrdered(ctx, in, func(ctx context.Context, x int) (int, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return 0, errors.New("timeout")
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}, Options{Workers: 1, TaskTimeout: 10 * time.Millisecond})

	r := <-out
	if r.Err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPanicAsError(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 1)
	in <- 1
	close(in)

	out := MapOrdered(ctx, in, func(_ context.Context, x int) (int, error) {
		panic("boom")
	}, Options{Workers: 1, PanicAsError: true})

	r := <-out
	if r.Err == nil {
		t.Fatal("expected panic error")
	}
	if _, ok := r.Err.(*PanicError); !ok {
		t.Fatalf("expected PanicError, got %T", r.Err)
	}
}

func TestNoGoroutineLeak(t *testing.T) {
	start := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan int)
	close(in)

	MapOrdered(ctx, in, func(_ context.Context, x int) (int, error) {
		return x, nil
	}, Options{Workers: 4})

	cancel()

	time.Sleep(10 * time.Millisecond)
	end := runtime.NumGoroutine()

	if end > start+1 {
		t.Fatalf("goroutine leak: %d -> %d", start, end)
	}
}

func BenchmarkMapOrdered(b *testing.B) {
	ctx := context.Background()
	data := make([]int, 1000)
	for i := range data {
		data[i] = i
	}

	b.Run("Sequential", func(b *testing.B) {
		for b.Loop() {
			input := make(chan int, len(data))
			for _, v := range data {
				input <- v
			}
			close(input)

			results := MapOrdered(ctx, input, func(ctx context.Context, x int) (int, error) {
				return x * 2, nil
			}, Options{Workers: 1})

			for range results {
				// Process results
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		for b.Loop() {
			input := make(chan int, len(data))
			for _, v := range data {
				input <- v
			}
			close(input)

			results := MapOrdered(ctx, input, func(ctx context.Context, x int) (int, error) {
				return x * 2, nil
			}, Options{Workers: 10})

			for range results {
				// Process results
			}
		}
	})
}

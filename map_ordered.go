package orderedpool

import (
	"context"
	"sync"
	"time"
)

type Result[R any] struct {
	Value R
	Err   error
}

type Options struct {
	Workers      int
	MaxInFlight  int
	EarlyStopN   int
	PanicAsError bool
	TaskTimeout  time.Duration
}

type indexedTask[T any] struct {
	index int
	value T
}

type indexedResult[R any] struct {
	index int
	res   Result[R]
}

func MapOrdered[T any, R any](
	ctx context.Context,
	input <-chan T,
	fn func(context.Context, T) (R, error),
	opt Options,
) <-chan Result[R] {
	if opt.Workers <= 0 {
		opt.Workers = 1
	}
	if opt.MaxInFlight < opt.Workers {
		opt.MaxInFlight = opt.Workers
	}

	output := make(chan Result[R], opt.MaxInFlight)

	go func() {
		defer close(output)

		// каналы для задач и результатов
		taskChan := make(chan indexedTask[T], opt.MaxInFlight)
		resultChan := make(chan indexedResult[R], opt.MaxInFlight)

		// worker pool
		var wg sync.WaitGroup
		for i := 0; i < opt.Workers; i++ {
			wg.Add(1)
			go worker(ctx, fn, opt, taskChan, resultChan, &wg)
		}

		// goroutine для закрытия каналов
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// главный цикл
		go func() {
			defer close(taskChan)

			index := 0
			for val := range input {
				select {
				case <-ctx.Done():
					return
				case taskChan <- indexedTask[T]{index: index, value: val}:
					index++
				}
			}
		}()

		// буфер для упорядочивания
		buffer := make(map[int]Result[R])
		nextIndex := 0
		successCount := 0

		for res := range resultChan {
			buffer[res.index] = res.res

			// публикация в порядке
			for {
				if item, ok := buffer[nextIndex]; ok {
					delete(buffer, nextIndex)
					select {
					case <-ctx.Done():
						return
					case output <- item:
						if item.Err == nil {
							successCount++
							if opt.EarlyStopN > 0 && successCount >= opt.EarlyStopN {
								return
							}
						}
					}
					nextIndex++
				} else {
					break
				}
			}
		}
	}()

	return output
}

func worker[T any, R any](
	ctx context.Context,
	fn func(context.Context, T) (R, error),
	opt Options,
	taskChan <-chan indexedTask[T],
	resultChan chan<- indexedResult[R],
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskChan:
			if !ok {
				return
			}

			// контекст задачи
			taskCtx := ctx
			var cancel context.CancelFunc
			if opt.TaskTimeout > 0 {
				taskCtx, cancel = context.WithTimeout(ctx, opt.TaskTimeout)
			}

			var res R
			var err error

			func() {
				defer func() {
					if r := recover(); r != nil {
						if opt.PanicAsError {
							err = &PanicError{Panic: r}
						} else {
							panic(r)
						}
					}
				}()
				res, err = fn(taskCtx, task.value)
			}()

			if cancel != nil {
				cancel()
			}

			select {
			case <-ctx.Done():
				return
			case resultChan <- indexedResult[R]{index: task.index, res: Result[R]{Value: res, Err: err}}:
			}
		}
	}
}

type PanicError struct {
	Panic any
}

func (e *PanicError) Error() string {
	return "panic occurred"
}

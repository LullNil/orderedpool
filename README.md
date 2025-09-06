# orderedpool

[![Go Report Card](https://goreportcard.com/badge/github.com/LullNil/orderedpool)](https://goreportcard.com/report/github.com/LullNil/orderedpool)
[![Go Reference](https://pkg.go.dev/badge/github.com/LullNil/orderedpool.svg)](https://pkg.go.dev/github.com/LullNil/orderedpool)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**orderedpool** is a generic Go library for parallel data processing with order preservation, context support, timeouts, and early termination.

## 🌟 Features

- **Order preservation** - Results are returned in the same order as input data
- **Limited worker pool** - Control over parallelism
- **Backpressure support** - Buffer limits via `MaxInFlight`
- **Early termination** - Stop after N successful results
- **Per-task timeouts** - Individual `context.WithTimeout` for each task
- **Panic handling** - Either as error or panic propagation
- **No goroutine leaks** - Proper cleanup on context cancellation
- **Thread-safe and race-free** - Passes `go test -race`

## 📦 Installation

```bash
go get github.com/LullNil/orderedpool
```

## 🚀 Quick Start

```go
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
        // Simulate work
        time.Sleep(time.Duration(x*100) * time.Millisecond)
        return fmt.Sprintf("result-%d", x), nil
    }, orderedpool.Options{
        Workers:      3,        // 3 parallel workers
        MaxInFlight:  5,        // max 5 tasks in flight
        EarlyStopN:   3,        // stop after 3 successful results
        PanicAsError: true,     // panics converted to errors
        TaskTimeout:  time.Second, // timeout per task
    })

    for res := range results {
        if res.Err != nil {
            fmt.Printf("Error: %v\n", res.Err)
        } else {
            fmt.Printf("Value: %s\n", res.Value)
        }
    }
}
```

## 📋 API

### `MapOrdered[T any, R any]`

```go
func MapOrdered[T any, R any](
    ctx context.Context,
    input <-chan T,
    fn func(context.Context, T) (R, error),
    opt Options,
) <-chan Result[R]
```

Processes input data in parallel while preserving order of results.

### `Options`

```go
type Options struct {
    Workers       int           // >0, number of workers
    MaxInFlight   int           // ≥ Workers, internal buffer limits
    EarlyStopN    int           // if >0, stop after N successful results
    PanicAsError  bool          // if true, panics are converted to errors
    TaskTimeout   time.Duration // if >0, deadline for each task
}
```

Configuration options for the worker pool.

### `Result[R any]`

```go
type Result[R any] struct {
    Value R
    Err   error
}
```

Result wrapper containing either a value or an error.

## 🛠️ Usage Examples

### 1. Parallel HTTP Requests

```go
results := orderedpool.MapOrdered(ctx, urls, func(ctx context.Context, url string) (string, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return string(body), nil
}, orderedpool.Options{
    Workers:     5,
    TaskTimeout: 10 * time.Second,
})
```

### 2. Data Processing with Early Termination

```go
// Find first 3 even numbers
results := orderedpool.MapOrdered(ctx, numbers, func(ctx context.Context, n int) (bool, error) {
    return n%2 == 0, nil
}, orderedpool.Options{
    Workers:    2,
    EarlyStopN: 3,
})
```

### 3. Heavy Computations with Timeout

```go
results := orderedpool.MapOrdered(ctx, data, func(ctx context.Context, item Data) (Result, error) {
    result, err := heavyComputation(item)
    return result, err
}, orderedpool.Options{
    Workers:      4,
    TaskTimeout:  30 * time.Second,
    PanicAsError: true,
})
```

### 4. Database Queries with Order Preservation

```go
type User struct {
    ID   int
    Name string
}

type UserDetail struct {
    User
    Details string
}

results := orderedpool.MapOrdered(ctx, userIDs, func(ctx context.Context, id int) (UserDetail, error) {
    user, err := getUserByID(ctx, id)
    if err != nil {
        return UserDetail{}, err
    }
    
    details, err := getUserDetails(ctx, id)
    if err != nil {
        return UserDetail{}, err
    }
    
    return UserDetail{User: user, Details: details}, nil
}, orderedpool.Options{
    Workers:     10,
    MaxInFlight: 20,
    TaskTimeout: 5 * time.Second,
})
```

## 🔧 Advanced Features

### Context Cancellation

The function properly handles context cancellation and ensures no goroutine leaks:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

results := orderedpool.MapOrdered(ctx, largeDataset, slowProcessingFunc, options)
// Will stop processing when context is cancelled
```

### Panic Handling

Control how panics are handled:

```go
// Convert panics to errors
results := orderedpool.MapOrdered(ctx, data, panickingFunc, orderedpool.Options{
    PanicAsError: true,
})

// Let panics propagate (default)
results := orderedpool.MapOrdered(ctx, data, panickingFunc, orderedpool.Options{
    PanicAsError: false,
})
```

### Memory Efficiency

The implementation uses bounded buffers to prevent memory issues:

```go
results := orderedpool.MapOrdered(ctx, infiniteStream, func(ctx context.Context, x int) (int, error) {
    return x * 2, nil
}, orderedpool.Options{
    Workers:     4,
    MaxInFlight: 10, // Prevents unbounded memory growth
})
```

## 🧪 Testing

```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Run coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📊 Performance Characteristics

- **Time Complexity**: O(n) where n is the number of input elements
- **Space Complexity**: O(MaxInFlight) for internal buffering
- **Parallelism**: Up to `Workers` tasks processed simultaneously
- **Order Preservation**: Guaranteed through indexed buffering

## 🐛 Known Limitations

1. **Memory Usage**: Buffer grows up to `MaxInFlight` elements for order preservation
2. **Blocking Input**: If input channel is unbuffered, it may block the producer
3. **Panic Propagation**: When `PanicAsError=false`, panics may not preserve exact stack trace

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/LullNil/orderedpool.git
cd orderedpool

# Run tests
go test -v ./...

# Run tests with race detector
go test -race ./...

# Check code coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 🎯 Use Cases

- **Web Scraping**: Parallel HTTP requests with order preservation
- **Data Processing Pipelines**: ETL processes with controlled parallelism
- **Microservice Orchestration**: Parallel API calls with timeouts
- **Database Operations**: Batch queries with connection pooling
- **Image/Video Processing**: Parallel processing with resource limits
- **Machine Learning**: Batch inference with order preservation

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details

**Made with ❤️ for the Go community**
# Log

This is a tiny library built for fitting Go services with logging functionality.

Logging is handled by the use of the Logger object which can be created using
```go
logger := log.New(logFile, logOptions)
```

Options for modes of logging include:
* ReshapeLogs: Reshapes the logs to wrap around the timestamp
* LogToFile: Allows to log to file
* LogToStdOut: Allows to log to std out

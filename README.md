logger
======

Package logger provides Logger4go which is a simple wrapper around go's log.Logger.

It provides three log handlers ConsoleHandler|FileHandler|SyslogHandler,
wrapper methods named after syslog's severity levels and embedds log.Logger to provide
seemless access to its methods as well if needed.

Supports:

    - Write to multiple handlers, e.g., log to console, file and syslog at the same time.
    - Use more than one logger instance. Each with its own set of handlers.
    - Log file rotation (size or daily) and compression.
    - Filter out severity levels.

Example output:

    main 2013/06/21 08:21:44.680513 -info- init called
    100m sprint 2013/06/21 08:21:44.680712 -info- Started 100m sprint: Should take 10 seconds.
    Long jump 2013/06/21 08:21:44.680727 -info- Started Long jump: Should take 6 seconds.
    High jump 2013/06/21 08:21:44.680748 -info- Started High jump: Should take 3 seconds.
    High jump 2013/06/21 08:21:47.683402 -info- Finished High jump
    Long jump 2013/06/21 08:21:50.683182 -info- Finished Long jump
    100m sprint 2013/06/21 08:21:54.683871 -info- Finished 100m sprint
    main 2013/06/21 08:22:14 -debug- A debug message
    main 2013/06/21 08:22:14 -info- An info message
    main 2013/06/21 08:22:14 -notice- A notice message
    main 2013/06/21 08:22:14 -warn- A warning message
    main 2013/06/21 08:22:14 -err- An error message
    main 2013/06/21 08:22:14 -crit- A critical message
    main 2013/06/21 08:22:14 -alert- An alert message
    main 2013/06/21 08:22:14 -emerg- An emergency message

TODO:
    - Custom header format
    - Read settings from config file

```go
    package logger_test

    import (
        "fmt"
        "github.com/alyu/logger"
        "log/syslog"
    )

    var lg *logger.Logger4go

    func Example() {
        // get a new logger instance named "example" and with prefix example
        lg = logger.Get("example")
        lg.Info("This is not written out, we need to add a handler first")

        // log to console/stdout
        lg.AddConsoleHandler()
        lg.Info("This will be written out to stdout")

        // log to file. as default the log will be rotated 5 times with a
        // max filesize of 1MB starting with sequence no 1, daily rotate and compression disabled
        _, err := lg.AddStdFileHandler("/tmp/logger.log")
        if err != nil {
            fmt.Errorf("%v", err)
        }
        lg.Alert("This is an alert message written to the console and log file")

        // log to syslog
        protocol := "" // tcp|udp
        ipaddr := ""
        sh, err := lg.AddSyslogHandler(protocol, ipaddr, syslog.LOG_INFO|syslog.LOG_LOCAL0, "example")
        if err != nil {
            fmt.Errorf("%v", err)
        }
        lg.Notice("This is a critical message written to the console, log file and syslog")
        lg.Notice("The format written to syslog is the same as for the console and log file")
        err = sh.Out.Err("This is a message to syslog without any preformatted header, it just contains this message")
        if err != nil {
            fmt.Errorf("%v", err)
        }

        // filter logs
        lg.SetFilter(logger.DEBUG | logger.INFO)
        lg.Alert("This message should not be shown")
        lg.Debug("This debug message is filtered through")
        lg.Info("As well as this info message")

        lg = logger.GetWithFlags("micro", logger.Ldate|logger.Ltime|logger.Lmicroseconds)
        lg.Info("This is written out with micrseconds precision")

        // get standard logger
        log := logger.Std()
        log.Info("Standard logger always has a console handler")

        // add a file handler which rotates 5 files with a maximum size of 5MB starting with sequence no 1, daily midnight rotation disabled
        // and with compress logs enabled
        log.AddFileHandler("/tmp/logger2.log", uint(5*logger.MB), 5, 1, true, false)

        // add a file handler which keeps logs for 5 days with no filesize limit starting with sequence no 1, daily midnight rotation
        // and  compress logs enabled
        log.AddFileHandler("/tmp/logger3.log", 0, 5, 1, true, true)

        // add a file handler with only one daily midnight rotation and compress logs enabled
        log.AddFileHandler("/tmp/logger3.log", 0, 1, 1, true, true)

        // Same as above
        fh, _ := log.AddStdFileHandler("/tmp/logger4.log")
        fh.SetSize(0)
        fh.SetRotate(1)
        fh.SetCompress(true)
        fh.SetDaily(true)
    }
```
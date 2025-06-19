# Fuzz testing

This uses native go fuzzing, which is available in Go 1.18 and later.
See https://go.dev/doc/security/fuzz/

## Initial setup

The initial corpus build by the original yaml-jsonpath maintainers in [go-fuzz](https://github.com/dvyukov/go-fuzz) format was generated using commands such as:

```shell
cd pkg/yamlpath/fuzz/testdata/fuzz/FuzzNewPath
grep 'path:' ../../../../lexer_test.go | grep -o '".*"' | sed 's/^"//' | sed 's/"$//' | awk '1==1{close("lexer_test"i);x="lexer_test"++i;}{print > x}'
grep 'selector:' ../../../../../../test/testdata/regression_suite.yaml | grep -o '".*"' | sed 's/^"//' | sed 's/"$//' | awk '1==1{close("regression_suite"i);x="regression_suite"++i;}{print > x}'
```

These files were then converted to the new format using `file2fuzz`:

```shell
go install golang.org/x/tools/cmd/file2fuzz@latest
file2fuzz -h
```

Example usage:
```shell
file2fuzz -o targetDirectory orignalDirectory/*
```

## Fuzzing

> Fuzzing is a type of automated testing which continuously manipulates inputs to a program to find bugs. Go fuzzing uses
> coverage guidance to intelligently walk through the code being fuzzed to find and report failures to the user. Since 
> it can reach edge cases which humans often miss, fuzz testing can be particularly valuable for finding security 
> exploits and vulnerabilities.
> - [Go Fuzzing](https://go.dev/doc/security/fuzz)

Fuzzing is invoked using the native `go test`. Fuzz tests for the predefined corpus will run alongside all other tests, but fuzzing
is not enabled by default. To run `FuzzNewPath` for 1 minute, use:

```shell
go test -fuzz=FuzzNewPath -fuzztime=1m -run=^$ ./pkg/yamlpath/fuzz
```

Or, to run the fuzz target 20000 times, use:

```shell
go test -fuzz=FuzzNewPath -fuzztime=20000x -run=^$ ./pkg/yamlpath/fuzz
```

You can tweak the options to fuzz. See `go help testflag` for details.

>   -fuzz regexp
>       Run the fuzz test matching the regular expression. When specified,
>       the command line argument must match exactly one package within the
>       main module, and regexp must match exactly one fuzz test within
>       that package. Fuzzing will occur after tests, benchmarks, seed corpora
>       of other fuzz tests, and examples have completed. See the Fuzzing
>
>       section of the testing package documentation for details.
>
>   -fuzztime t
>       Run enough iterations of the fuzz target during fuzzing to take t,
>       specified as a time.Duration (for example, -fuzztime 1h30s).
>           The default is to run forever.
>
>       The special syntax Nx means to run the fuzz target N times
>       (for example, -fuzztime 1000x).
>
>   -fuzzminimizetime t
>       Run enough iterations of the fuzz target during each minimization
>       attempt to take t, as specified as a time.Duration (for example,
>       -fuzzminimizetime 30s).
>
>   -parallel n
>       Allow parallel execution of test functions that call t.Parallel, and
>       fuzz targets that call t.Parallel when running the seed corpus.
>       The value of this flag is the maximum number of tests to run
>       simultaneously.
>       While fuzzing, the value of this flag is the maximum number of
>       subprocesses that may call the fuzz function simultaneously, regardless of
>       whether T.Parallel is called.
>       By default, -parallel is set to the value of GOMAXPROCS.
>       Setting -parallel to values higher than GOMAXPROCS may cause degraded
>       performance due to CPU contention, especially when fuzzing.

## Entertainment

I used [watchman](https://facebook.github.io/watchman/) to print out new corpus as it's found:
```
cd pkg/yamlpath/fuzz/corpus
watchman watch $PWD
watchman -- trigger $PWD buildme '*' -- cat
tail -f /usr/local/var/run/watchman/*/log
```

You're log location may vary - see [stack overflow](https://stackoverflow.com/questions/27723367/watchman-where-is-the-default-log-file).
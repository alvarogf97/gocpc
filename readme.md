<p align="center">
  <img width="250" height="250" src="https://static.thenounproject.com/png/2103954-200.png">
</p>


# Gocpc in a nutshell

Gocpc is a cli tool that allow you to request in a parallelized way the spain bankruptcy advertising portal. Moreover it can handle large csv files as the input reading it line to line without collapse the computer memory so it works on low memory computers.

# Install

You can install either from source or binaries directly

## From source

### Windows

```cmd
make build.windows
```

### Linux

```cmd
make build.linux
```

## Using binaries

### Windows

> **_NOTE:_**  Open CMD as administrator for moving the file to system32 folder

```cmd
curl -LO https://github.com/alvarogf97/gocpc/releases/download/v0.0.1/gocpc.exe
move gocpc.exe \Windows\system32\gocpc.exe
```

### Linux

```cmd
curl https://github.com/alvarogf97/gocpc/releases/download/v0.0.1/gocpc.exe --output gocpc
mv gocpc /usr/bin
```

# How to use it

As a cli tool you can always use de `--help` flag to make sure what commands do

### Process

Process command takes a csv as the input and process every row document by searching it in the spain bankruptcy advertising portal

```cmd
> gocpc process -h

Process csv file and request CPC for every document record found in it

Usage:
  gocpc process [flags]

Flags:
  -h, --help                  help for process
  -i, --input string          input file (default "data.csv")
  -l, --log string            errors log file (default "errors.log")
  -o, --output string         output file (default "matches.csv")
  -r, --request-retries int   times failed request will be retried (default 1)
  -s, --starts-from int       starts from (default 1)
  -t, --threads int           threads to use (default 1)
```
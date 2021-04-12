# gosn-v2

[![Build Status](https://www.travis-ci.org/jonhadfield/gosn-v2.svg?branch=master)](https://www.travis-ci.org/jonhadfield/gosn-v2) [![CircleCI](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master.svg?style=svg)](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/jonhadfield/gosn-v2/) [![Go Report Card](https://goreportcard.com/badge/github.com/jonhadfield/gosn-v2)](https://goreportcard.com/report/github.com/jonhadfield/gosn-v2)

This release adds support for version 004 of encryption (introduced Nov 2020) and removes support for 003.  

####Note: This is an early release with significant changes. Please take a backup before using with any real data and report any issues you find. 

## about
<a href="https://standardnotes.org/" target="_blank">Standard Notes</a> is a service and application for the secure
management and storage of notes.

gosn-v2 is a library to help develop your own application to manage notes on the official, or your self-hosted, Standard
Notes server.


## documentation

- [guides](docs/index.md)
- [go docs](https://pkg.go.dev/github.com/jonhadfield/gosn-v2)

## basic usage

The following example shows how to use this library to interact with the SN API directly. To prevent a full download of all items on every sync, use the provided [cache package](cache/README.md) to persist an encrypted copy to disk, only syncing deltas on subsequent calls.

## installation

```bash
GO111MODULE=on go get -u github.com/jonhadfield/gosn-v2
```

## importing

```go
import "github.com/jonhadfield/gosn-v2"
```

## basic usage

### authenticating

Sign-in to obtain your session

```go
sio, _ := gosn.SignIn(gosn.SignInInput{
    Email:    "user@example.com",
    Password: "topsecret",
})
```

### initial sync

An initial sync is required to retrieve your items, as well as your Items Keys that are then added to your session

```go
so, _ := gosn.Sync(gosn.SyncInput{
    Session: &sio.Session,
})
```

### decrypt and parse items

```go
items, _ := so.Items.DecryptAndParse(&sio.Session)
```


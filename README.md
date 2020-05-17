# gosn-v2
[![Build Status](https://www.travis-ci.org/jonhadfield/gosn-v2.svg?branch=master)](https://www.travis-ci.org/jonhadfield/gosn-v2) [![CircleCI](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master.svg?style=svg)](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/jonhadfield/gosn-v2/) [![Go Report Card](https://goreportcard.com/badge/github.com/jonhadfield/gosn-v2)](https://goreportcard.com/report/github.com/jonhadfield/gosn-v2) 

# about
<a href="https://standardnotes.org/" target="_blank">Standard Notes</a> is a service and application for the secure management and storage of notes.  

gosn is a library to help develop your own application to manage notes on the official, or your self-hosted, Standard Notes server.

# installation

Using go get: ``` go get github.com/jonhadfield/gosn-v2```

# documentation

- [guides](docs/index.md)
- [go docs](https://godoc.org/github.com/jonhadfield/gosn-v2)

# basic usage
## authenticating

To interact with Standard Notes you first need to sign in:

```golang
    sIn := gosn.SignInInput{
        Email:     "someone@example.com",
        Password:  "mysecret,
    }
    sOut, _ := gosn.SignIn(sIn)
```

This will return a session containing the necessary secrets and information to make requests to get or put data.

## getting items

```golang
    input := GetItemsInput{
        Session: sOut.Session,
    }
    gio, _ := GetItems(input)
```

## creating a note

```golang
    # create note content
    content := NoteContent{
        Title:          "Note Title",
        Text:           "Note Text",
    }
    # create note
    note := NewNote()
    note.Content = content
    
    # sync note
    pii := PutItemsInput{
    		Session: sOut.Session,
    		Items:   []gosn.Notes{note},
    }
    pio, _ := PutItems(pii)
```



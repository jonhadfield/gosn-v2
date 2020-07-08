# gosn-v2
[![Build Status](https://www.travis-ci.org/jonhadfield/gosn-v2.svg?branch=master)](https://www.travis-ci.org/jonhadfield/gosn-v2) [![CircleCI](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master.svg?style=svg)](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/jonhadfield/gosn-v2/) [![Go Report Card](https://goreportcard.com/badge/github.com/jonhadfield/gosn-v2)](https://goreportcard.com/report/github.com/jonhadfield/gosn-v2) 

# about
<a href="https://standardnotes.org/" target="_blank">Standard Notes</a> is a service and application for the secure management and storage of notes.  

gosn is a library to help develop your own application to manage notes on the official, or your self-hosted, Standard Notes server.

# installation

```go get github.com/jonhadfield/gosn-v2```

# documentation

- [guides](docs/index.md)
- [go docs](https://pkg.go.dev/github.com/jonhadfield/gosn-v2)

# basic usage
## authenticating

To interact with Standard Notes you first need to sign in:

```golang
si := gosn.SignInInput{
    Email:     "someone@example.com",
    Password:  "mysecret,
}

so, _ := gosn.SignIn(si)
```

This will return a session containing the necessary secrets and information to make requests to get or put data.

## getting items

```golang
input := gosn.SyncInput{
    Session: so.Session,
}

gio, _ := gosn.Sync(input)
```

## decrypt and parse items
```golang
di, _ := gio.Items.DecryptAndParse(so.Session.Mk, so.Session.Ak, false)
```

## output items
```golang
for _, tag := range di.Tags() {
    fmt.Println(tag.Content.Title)
}

for _, note := range di.Notes() {
    fmt.Println(note.Content.Title, note.Content.Text)
}
```

## creating a note

```golang
// create note content
content := gosn.NoteContent{
    Title: "Note Title",
    Text:  "Note Text",
}

// create note
note := gosn.NewNote()
note.Content = content

// encrypt
notes := gosn.Notes{note}
en, _ := notes.Encrypt(so.Session.Mk, so.Session.Ak, false)
    
// sync
gosn.Sync(gosn.SyncInput{
	Session: so.Session,
	Items:   en,
})
```



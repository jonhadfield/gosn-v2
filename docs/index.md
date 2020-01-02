# guides
- [registration](#registration)
- [authentication](#authentication)
- [syncing](#syncing)


## registration

The registration flow is documented [here](https://standardfile.org/#api-auth).

```golang
    rIn := gosn.RegisterInput{
        Email:     "someone@example.com",
        Password:  "mysecret",
    }
    rOut, err := rIn.Register()
```

## authentication

The authentication flow is documented [here](https://standardfile.org/#api-auth).

Authentication always requires an identifier (email address) and password.  
If MFA (Multi Factor Authentication) has been set then a second request needs to be made that includes the token name and value.  

### without MFA 

```golang
    sIn := gosn.SignInInput{
        Email:     "someone@example.com",
        Password:  "mysecret,
    }
    sOut, err := gosn.SignIn(sIn)
```

### with MFA

Make the same initial as above. If MFA is configured for the user:
- an error will be returned containing string "requestMFA"
- the returned SignInOutput struct field tokenName' will be set to the name of the registered token

With that information, make a new authentication request with the token name and the token value, e.g.:

```golang
    sIn.TokenName = sOut.TokenName
    sIn.TokenVal = <token value>
    sOut, err = gosn.SignIn(sIn) 
```

### authentication output

Successful authentication results in a SignInOutput struct containing a Session entry. 
The session needs to be passed with all calls to 'sync' (get and put) notes and other items.

## syncing

Syncing is the process of getting and putting items

### getting items

To retrieve all items, create an input struct with the session returned from authentication:

```golang
gii := gosn.GetItemsInput{
    Session: <session returned from authentication>,
}
```
and make the request:
```golang
gio, err := gosn.GetItems(getItemsInput)
```

### putting items

create an item:
```golang

note := gosn.NewNote()
noteContent := gosn.NewNoteContent()
noteContent.Title = "My note"
noteContent.Text = "some content"
```
and put it:
```golang
pii := gosn.PutItemsInput{
    Session: <session returned from authentication>,
    Items:   []gosn.Item{*newNote},
}
pio, err := gosn.PutItems(pii)
```

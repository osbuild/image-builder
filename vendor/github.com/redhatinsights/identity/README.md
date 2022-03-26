# Identity

This package contains the common identity document data structure as well as go
http middleware for handling an incoming identity document.

## How to use the middleware

If you are using the vanilla go mux you can wrap your handlers with the provided Handlers.

```
http.Handle("/", identity.Extract(myHttpHandler))
```

If you are using a system like go-chi you might do something like this:

```
r := chi.Router()
r.Use(identity.Extract)
```

Assuming you are usin the middleware the structure can be retrieved from the context in any given HandlerFunc.

```
func myHandler(w http.ResponseWriter, r *http.Request) {
    ident, ok := identity.Get(r.Context())
    if !ok {
        // no auth doc, bail out
        return
    }
    // do something with the ident
}
```

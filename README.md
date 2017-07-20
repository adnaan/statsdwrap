 Usage:

  ```go
  r := chi.NewRouter()
  statsdClient, _ := statsd.New(
    statsd.Prefix("myapp"),
    statsd.Address("localhost:8125"),
  )
  wrap := statsdwrap.NewChi("user_service", statsdClient)
  handleHome := func(w http.ResponseWriter, r *http.Request) {
  	time.Sleep(time.Millisecond * 1000)
  	w.WriteHeader(http.StatusOK)
  	w.Write([]byte("OK"))
  }
  r.Get(wrap.HandlerFunc("home", "/", handleHome))
```
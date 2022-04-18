# test

```
# tcp listener
nc -l -p 3000

# proxy client
./client --target 127.0.0.1:3000  --source 127.0.0.1:2203

# udp client
nc  -u 127.0.0.1 2203
```

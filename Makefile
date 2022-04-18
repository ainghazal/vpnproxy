listen-server:
	nc -n -v -l -u -p 4000
test-server:
	cd server && go build && ./server --source 127.0.0.1:3000 --target 127.0.0.1:4000

test-client:
	cd client && go build && ./client --target  127.0.0.1:3000 --source 127.0.0.1:2203

dial-client:
	nc -u 127.0.0.1 2203

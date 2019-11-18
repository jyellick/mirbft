## To execute

First, within this directory, build and start the demo mirbft server with:

### `go build . && ./demo`

This starts a web server running on port 10000 which proxies requests to a fake network of mirbft nodes.

Then, in this same directory, run 

### `npm install && npm start dev`

Note, it is important to run with 'dev' as the node server will be proxying requests through to tbe backing golang one.  This should launch a browser, but otherwise, you may point to [http://localhost:3000](http://localhost:3000) to view it in the browser.

Note: This project was bootstrapped with [Create React App](https://github.com/facebook/create-react-app).

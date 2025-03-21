package main

//
// A CLI + Server that uses the "pluto" module to expose all of the data structures.
// i.e. (Redis like shared storage server)
//
// Interfaces:
// 		CLI
// 		https, http - restful api
// 		websocket
// 		gRPC
//
// An internal LUA languge is used as the scripting language.
//
// Authentication
//		xyzzy...
//
//
// $ hydra [ options ] [ cmd ]
//
//	Options:
//		--port
//		--host
//		--keys_dir
//		--pub_key
//		--priv_key
//		--debug_flags
//		--cfg
//		--username
//		--init						Create a default hydra-cfg.json, this will error if hydra-cfg.json exists.
//
//		--http :port
//		--https :port
//		--websocket :port
//		--gRPC :port
//
//		--reload-cfg				Send message to server to reload config file.
//
// 	cmd
//		version
//		cli
//		server
//		web-tool					Run a website with auth for playing with / developing with pluto+hydra (full website for this)
//									- Explore / Watch data - with graphical output
//									- monitor changes
//

//
// RESTful API
// ===========================================================================================
//
// API Version 1
//
// set/get/type/del/ttl/setttl have an optional db=Name
//
// Set
// ---
//  CLI: set a b [Db]
//
// 	HTTP(s): /av1/set/a/b?db=Name
//	HTTP(s): /av1/set?name=a&value=b&db=Name
//
//	gRPC: set
//
//  WebSocket: cmd="/av1/set"
//
// Get
// ---
// 	HTTP(s): /av1/set/a/b?db=Name
//	HTTP(s): /av1/set?name=a&value=b&db=Name
//
// Type
// ----
//
// Del
// ---
//
// TTL
// ---
//
// SetTTL
// ------
//
// Database
// --------
//
//

func main() {
}

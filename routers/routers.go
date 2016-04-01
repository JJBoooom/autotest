package routers

import (
	"net/http"
	"test/handler"

	"github.com/gorilla/mux"
)

type Routes []Route

type Route struct {
	Name    string
	Method  string
	Pattern string
	Handler http.Handler
}

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	//	router.NotFoundHandler = http.HandlerFunc(handler.NotFound)
	//routes = append(routes, nsRoutes...)
	//router.Handle()

	for _, route := range routes {
		//同时存在HandlerFunc、Handler会有什么问题?
		//哪个在后面，哪个被设置
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.Handler)
		//router.Handle
	}
	return router
}

var routes = Routes{
	/*
		Route{
			Name:    "Logout",
			Pattern: "/logout",
			Method:  "GET",
			Handler: negroni.New(
				negroni.Handler(handler.NegJsonReturnHandler(handler.RequireAuthToken)),
				negroni.Handler(handler.NegJsonReturnHandler(handler.LogoutHandler)),
			),
		},
		Route{
			Name:    "Login",
			Pattern: "/login",
			Method:  "GET",
			Handler: handler.JsonReturnHandler(handler.Login),
		},
	*/
	Route{
		Name:    "Images",
		Pattern: "/list",
		Method:  "GET",
		Handler: handler.JsonReturnHandler(handler.ListImages),
	},

	Route{
		Name:    "Images",
		Pattern: "/get",
		Method:  "GET",
		Handler: handler.JsonReturnHandler(handler.PullImage),
	},
}

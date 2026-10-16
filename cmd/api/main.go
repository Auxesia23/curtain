package main

import (
	"flag"
	"fmt"
	"log"

	"curtain/internal/authz"
	"curtain/internal/database"
	"curtain/internal/handlers"
	"curtain/internal/middleware"
	"curtain/internal/routes"
	"curtain/internal/services"

	curtain "curtain"
)

const serverVersion = "0.1.0"

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "./curtain.db", "SQLite database file path")
	cookieSecure := flag.String("cookie-secure", "auto", "session cookie Secure mode: auto, true, or false")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("curtain-server %s\n", serverVersion)
		return
	}

	secureMode := handlers.CookieSecureMode(*cookieSecure)
	if secureMode != handlers.CookieSecureAuto && secureMode != handlers.CookieSecureTrue && secureMode != handlers.CookieSecureFalse {
		log.Fatalf("invalid --cookie-secure value %q (expected auto, true, or false)", *cookieSecure)
	}

	db, err := database.Connect(*dbPath)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("database handle: %v", err)
	}
	defer sqlDB.Close()

	enforcer, err := authz.NewEnforcer(db)
	if err != nil {
		log.Fatalf("authorization setup: %v", err)
	}

	authSvc := services.NewAuthService(db)
	inviteSvc := services.NewInviteService(db)
	userSvc := services.NewUserService(db, enforcer)
	projectSvc := services.NewProjectService(db, enforcer)
	envSvc := services.NewEnvironmentService(db)
	varSvc := services.NewVariableService(db)

	authH := handlers.NewAuthHandler(authSvc, secureMode)
	inviteH := handlers.NewInviteHandler(inviteSvc)
	userH := handlers.NewUserHandler(userSvc)
	projectH := handlers.NewProjectHandler(projectSvc)
	envH := handlers.NewEnvironmentHandler(envSvc)
	varH := handlers.NewVariableHandler(varSvc)
	permissionH := handlers.NewPermissionHandler()
	authzMiddleware := middleware.NewAuthorize(db, enforcer)
	authn := middleware.NewAuthenticator(db)

	router := routes.NewRouter(authH, inviteH, userH, projectH, envH, varH, permissionH, authzMiddleware, authn, curtain.SPAHandler())

	if needsSetup, err := authSvc.NeedsSetup(); err != nil {
		log.Printf("curtain: failed to check instance status: %v", err)
	} else if needsSetup {
		log.Println("curtain: WARNING — no users exist. The first person to open the web UI will become the instance admin.")
	}

	srv := newServer(config{addr: *addr}, router)
	log.Printf("listening on %s (database: %s)", *addr, *dbPath)
	log.Fatal(srv.run())
}

package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/markbates/goth/gothic"
)

func AuthRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        session, err := gothic.Store.Get(c.Request, "user-session")
        if err != nil || session.Values["user_id"] == nil {
            c.Redirect(302, "/login")
            c.Abort()
            return
        }
        c.Set("user_id", session.Values["user_id"])
        c.Next()
    }
}
package identity

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

type registerRequest struct {
	Name     string    `json:"name"     binding:"required,max=255"`
	Email    string    `json:"email"    binding:"required,email,max=255"`
	Password string    `json:"password" binding:"required"`
	Role     auth.Role `json:"role"     binding:"required"`
}

func (h *Handler) register(c *gin.Context) error {
	req, err := web.Bind[registerRequest](c)
	if err != nil {
		return err
	}

	u, err := h.service.Register(c.Request.Context(), RegisterInput(req))
	if err != nil {
		return err
	}

	c.JSON(http.StatusCreated, u)
	return nil
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) login(c *gin.Context) error {
	req, err := web.Bind[loginRequest](c)
	if err != nil {
		return err
	}

	pair, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	writeTokens(c, pair)
	return nil
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) refresh(c *gin.Context) error {
	req, err := web.Bind[refreshRequest](c)
	if err != nil {
		return err
	}

	pair, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		return err
	}

	writeTokens(c, pair)
	return nil
}

func (h *Handler) logout(c *gin.Context) error {
	req, err := web.Bind[refreshRequest](c)
	if err != nil {
		return err
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		return err
	}

	c.Status(http.StatusNoContent)
	return nil
}

// Token responses must not be cached (RFC 6749 section 5.1).
func writeTokens(c *gin.Context, pair TokenPair) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, pair)
}

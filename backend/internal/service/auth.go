package service

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// refreshCookieName and refreshHeaderName are the two accepted transports for
// the opaque refresh token: an httpOnly cookie scoped to /v1/auth (browsers),
// or a plain header (non-cookie clients such as CLI tools and gRPC).
const (
	refreshCookieName = "refresh_token"
	refreshHeaderName = "X-Refresh-Token"
	refreshCookiePath = "/v1/auth"
)

// AuthService is an auth service.
type AuthService struct {
	v1.UnimplementedAuthServiceServer

	uc     *biz.AuthUsecase
	wechat *biz.WechatUsecase
}

// NewAuthService new an auth service.
func NewAuthService(uc *biz.AuthUsecase, wechat *biz.WechatUsecase) *AuthService {
	return &AuthService{uc: uc, wechat: wechat}
}

// Register creates a READER account and issues the token pair, delivering
// the refresh token as an httpOnly cookie — the caller is signed in exactly
// as if they had logged in.
func (s *AuthService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.LoginReply, error) {
	u, pair, err := s.uc.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetDisplayName(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	setRefreshCookie(ctx, pair.Refresh.Token, time.Until(pair.Refresh.ExpiresAt))
	return convertLoginReply(u, pair), nil
}

// Login verifies credentials and issues the token pair, delivering the
// refresh token as an httpOnly cookie.
func (s *AuthService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	u, pair, err := s.uc.Login(ctx, req.GetEmail(), req.GetPassword(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	setRefreshCookie(ctx, pair.Refresh.Token, time.Until(pair.Refresh.ExpiresAt))
	return convertLoginReply(u, pair), nil
}

// Refresh rotates the refresh session and issues a new pair; the replacement
// refresh cookie is set on the way out.
func (s *AuthService) Refresh(ctx context.Context, req *v1.RefreshRequest) (*v1.LoginReply, error) {
	token, err := refreshTokenFrom(ctx)
	if err != nil {
		return nil, err
	}
	u, pair, err := s.uc.Refresh(ctx, token)
	if err != nil {
		return nil, err
	}
	setRefreshCookie(ctx, pair.Refresh.Token, time.Until(pair.Refresh.ExpiresAt))
	return convertLoginReply(u, pair), nil
}

// Logout revokes the refresh session and clears the cookie. Idempotent: a
// logout without a valid session still clears the cookie and succeeds.
func (s *AuthService) Logout(ctx context.Context, req *v1.LogoutRequest) (*emptypb.Empty, error) {
	if token, err := refreshTokenFrom(ctx); err == nil {
		if err := s.uc.Logout(ctx, token); err != nil {
			return nil, err
		}
	}
	clearRefreshCookie(ctx)
	return &emptypb.Empty{}, nil
}

// GetMe returns the account behind the verified access token.
func (s *AuthService) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.User, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	u, err := s.uc.CurrentUser(ctx, claims)
	if err != nil {
		return nil, err
	}
	return convertUser(u), nil
}

// convertLoginReply builds the shared Login/Refresh response body. The
// refresh token is never inlined: it travels as a cookie or header only.
func convertLoginReply(u *biz.User, pair *biz.TokenPair) *v1.LoginReply {
	return &v1.LoginReply{
		AccessToken: pair.Access.Token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(pair.Access.ExpiresAt).Seconds()),
		User:        convertUser(u),
	}
}

func convertUser(u *biz.User) *v1.User {
	if u == nil {
		return nil
	}
	return &v1.User{
		Id:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        convertUserRole(u.Role),
		AvatarUrl:   u.AvatarURL,
		CreatedAt:   timestamppb.New(u.CreatedAt),
	}
}

// convertUserRole maps a domain role onto the api enum. The two share their
// numeric values, but the mapping is written out so neither side can drift
// into the other silently.
func convertUserRole(in biz.UserRole) v1.UserRole {
	switch in {
	case biz.UserRoleAdmin:
		return v1.UserRole_USER_ROLE_ADMIN
	case biz.UserRoleReader:
		return v1.UserRole_USER_ROLE_READER
	default:
		return v1.UserRole_USER_ROLE_UNSPECIFIED
	}
}

// refreshTokenFrom reads the refresh token from the cookie first and falls
// back to the X-Refresh-Token header for non-cookie clients. Missing tokens
// surface as the domain unauthorized error, not as transport details.
func refreshTokenFrom(ctx context.Context) (string, error) {
	if req, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		if cookie, err := req.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		if header := strings.TrimSpace(tr.RequestHeader().Get(refreshHeaderName)); header != "" {
			return header, nil
		}
	}
	return "", biz.ErrAuthInvalidRefreshToken
}

// setRefreshCookie delivers the opaque refresh token. The cookie is httpOnly
// and scoped to /v1/auth so neither scripts nor article endpoints ever see
// it; SameSite=Lax fits the same-origin Nuxt proxy. Secure stays off until
// the site ships behind TLS.
func setRefreshCookie(ctx context.Context, token string, maxAge time.Duration) {
	w, ok := kratoshttp.ResponseWriterFromServerContext(ctx)
	if !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearRefreshCookie expires the refresh cookie in place; the Path must match
// the one it was set with for browsers to drop it.
func clearRefreshCookie(ctx context.Context) {
	w, ok := kratoshttp.ResponseWriterFromServerContext(ctx)
	if !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP returns the best-effort caller address for rate limiting. Behind
// the proxy chain (Caddy → BFF → backend) every trusted hop APPENDS to
// X-Forwarded-For, so the rightmost entry is the client address as seen by
// the proxy we control — the leftmost entry is fully client-controlled and
// taking it would hand out a fresh throttle bucket per request. Without the
// header the TCP peer is used. Never used as an identity — only as a
// throttle key.
func clientIP(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		if ip := rightmostForwardedIP(tr.RequestHeader().Get("X-Forwarded-For")); ip != "" {
			return ip
		}
	}
	if req, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			return host
		}
		return req.RemoteAddr
	}
	return "unknown"
}

// rightmostForwardedIP returns the last non-empty entry of a comma-separated
// X-Forwarded-For list; empty input yields "".
func rightmostForwardedIP(xff string) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if ip := strings.TrimSpace(parts[i]); ip != "" {
			return ip
		}
	}
	return ""
}

// WechatLogin signs a mini-program user in with a wx.login code: bound
// openids receive the standard token pair (refresh cookie set), unbound ones
// receive a single-use binding ticket.
func (s *AuthService) WechatLogin(ctx context.Context, req *v1.WechatLoginRequest) (*v1.WechatLoginReply, error) {
	res, err := s.wechat.Login(ctx, req.GetCode(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	if res.Pair != nil {
		setRefreshCookie(ctx, res.Pair.Refresh.Token, time.Until(res.Pair.Refresh.ExpiresAt))
		return &v1.WechatLoginReply{
			Status: v1.WechatLoginStatus_WECHAT_LOGIN_STATUS_OK,
			Login:  convertLoginReply(res.User, res.Pair),
		}, nil
	}
	return &v1.WechatLoginReply{
		Status:        v1.WechatLoginStatus_WECHAT_LOGIN_STATUS_BINDING_REQUIRED,
		BindingTicket: res.BindingTicket,
	}, nil
}

// BindWechat attaches the ticket's openid to an existing account and signs
// the caller in with the standard token pair and refresh cookie.
func (s *AuthService) BindWechat(ctx context.Context, req *v1.BindWechatRequest) (*v1.LoginReply, error) {
	u, pair, err := s.wechat.Bind(ctx, req.GetBindingTicket(), req.GetEmail(), req.GetPassword(), clientIP(ctx))
	if err != nil {
		return nil, err
	}
	setRefreshCookie(ctx, pair.Refresh.Token, time.Until(pair.Refresh.ExpiresAt))
	return convertLoginReply(u, pair), nil
}

// UpdatePassword rotates the calling account's password. The account
// identity comes from the verified access token; the policy layer already
// restricts the route to authenticated subjects, and the usecase re-checks
// the claims as defense in depth.
func (s *AuthService) UpdatePassword(ctx context.Context, req *v1.UpdatePasswordRequest) (*emptypb.Empty, error) {
	claims, ok := biz.AuthFromContext(ctx)
	if !ok {
		return nil, biz.ErrAuthUnauthorized
	}
	if err := s.uc.UpdatePassword(ctx, claims.UserID, req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

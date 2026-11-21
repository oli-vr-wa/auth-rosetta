from fastapi import APIRouter, Request, Response
from fastapi.responses import JSONResponse

from app.api.cookies import clear_auth_cookie, set_auth_cookie
from app.api.deps import AuthServiceDep, CurrentClaims
from app.api.schemas import RegisterRequest, ConfirmRequest, LoginRequest

router = APIRouter(prefix="/api/auth")

@router.post("/register")
def register(body: RegisterRequest, auth: AuthServiceDep) -> Response:
    result = auth.register(body.email, body.password)
    if result is None:
        return JSONResponse({"error": "Email already registered"}, status_code=409)
    return JSONResponse({"otpauthUri": result.otpauth_uri, "secret": result.secret}, status_code=201)


@router.post("/2fa/confirm")
def confirm(body: ConfirmRequest, auth: AuthServiceDep) -> Response:
    if not auth.confirm_two_factor(body.email, body.code):
        return JSONResponse({"error": "Invalid code"}, status_code=400)
    return JSONResponse({"message": "Two-factor authentication enabled"})


@router.post("/login")
def login(body: LoginRequest, request: Request, auth: AuthServiceDep) -> Response:
    token = auth.login(body.email, body.password, body.code)
    if token is None:
        return JSONResponse({"error": "Invalid credentials"}, status_code=401)
    response = JSONResponse({"message": "Login successful"})
    set_auth_cookie(response, request, token)
    return response


@router.get("/me")
def me(claims: CurrentClaims) -> Response:
    return JSONResponse({"email": claims.email})


@router.post("/logout")
def logout(request: Request, claims: CurrentClaims, auth: AuthServiceDep) -> Response:
    auth.logout(claims.jti, claims.expires_at)
    response = Response(status_code=204)
    clear_auth_cookie(response, request)
    return response
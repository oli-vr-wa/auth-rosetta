export interface RegisterResponse {
    otpauthUri: string;
    secret: string;
}

export interface MessageResponse {
    message: string;
}

export interface LoginResponse {
    token: string;
}

export interface MeResponse {
    email: string;
}
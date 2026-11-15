import { Module } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtModule } from '@nestjs/jwt';
import { JWT_AUDIENCE, JWT_EXPIRY_MINUTES, JWT_ISSUER } from './auth.constants';
import { PasswordService } from './password.service';
import { TokenService } from './token.service';
import { TotpService } from './totp.service'; 
import { AuthController } from './auth.controller';
import { AuthService } from './auth.service';
import { TypeOrmModule } from '@nestjs/typeorm';
import { User } from '@/users/user.entity';
import { RevokedToken } from './revoked-token.entity';

@Module({
    imports: [
        TypeOrmModule.forFeature([User]),
        TypeOrmModule.forFeature([RevokedToken]),
        JwtModule.registerAsync({
            inject: [ConfigService],
            useFactory: (config: ConfigService) => ({
                secret: config.getOrThrow<string>('JWT_KEY'),
                signOptions: {
                    algorithm: 'HS256',
                    issuer: JWT_ISSUER,
                    audience: JWT_AUDIENCE,
                    expiresIn: JWT_EXPIRY_MINUTES * 60 // Convert minutes to seconds
                },
            }),
        }),
    ],
    controllers: [AuthController],
    providers: [PasswordService, TokenService, TotpService, AuthService],
    exports: [PasswordService, TokenService, TotpService],
})
export class AuthModule {}
import { Module } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtModule } from '@nestjs/jwt';
import { JWT_AUDIENCE, JWT_EXPIRY_MINUTES, JWT_ISSUER } from './auth.constants';
import { PasswordService } from './password.service';
import { TokenService } from './token.service';
import { TotpService } from './totp.service'; 

@Module({
    imports: [
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
    providers: [PasswordService, TokenService, TotpService],
    exports: [PasswordService, TokenService, TotpService],
})
export class AuthModule {}
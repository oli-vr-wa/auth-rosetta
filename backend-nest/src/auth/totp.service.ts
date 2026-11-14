import { Injectable } from '@nestjs/common';
import {
    generateSecret as otpGenerateSecret,
    generateURI,
    verify as otpVerify
} from 'otplib';
import { TOTP_ISSUER } from './auth.constants';

@Injectable()
export class TotpService {
    createSecret(): string {
        return otpGenerateSecret();
    }

    buildUri(email: string, secret: string): string {
        return generateURI({ issuer: TOTP_ISSUER, label: email, secret });
    }

    async verify(secret: string, code: string): Promise<boolean> {
        try {
            const result = await otpVerify({ secret, token: code, epochTolerance: 30 });
            return result.valid;
        } catch {
            // A malformed code will throw an error, so we catch it and return false.
            return false;
        }
    }
}
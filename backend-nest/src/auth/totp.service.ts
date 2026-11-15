import { Injectable } from '@nestjs/common';
import {
    generateSecret as otpGenerateSecret,
    generateURI,
    verify as otpVerify
} from 'otplib';
import { TOTP_ISSUER } from './auth.constants';

@Injectable()
export class TotpService {

    /**
     * Generates a new TOTP secret.
     * @returns {string} The generated TOTP secret.
     */
    createSecret(): string {
        return otpGenerateSecret();
    }

    /**
     * Builds a TOTP URI for the given email and secret.
     * @param {string} email - The email address associated with the TOTP.
     * @param {string} secret - The TOTP secret.
     * @returns {string} The generated TOTP URI.
     */
    buildUri(email: string, secret: string): string {
        return generateURI({ issuer: TOTP_ISSUER, label: email, secret });
    }

    /**
     * Verifies a TOTP code against the provided secret.
     * @param {string} secret - The TOTP secret.
     * @param {string} code - The TOTP code to verify.
     * @returns {Promise<boolean>} A promise that resolves to true if the code is valid, false otherwise.
     */
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
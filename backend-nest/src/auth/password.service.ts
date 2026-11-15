import { Injectable } from '@nestjs/common';
import * as bcrypt from 'bcrypt';
import { BCRYPT_COST } from './auth.constants';

@Injectable()
export class PasswordService {

    /**
     * Hashes the provided password using bcrypt with a specified cost factor.
     * @param password - The plain text password to be hashed.
     * @returns A promise that resolves to the hashed password.
     */
    hash(password: string): Promise<string> {
        return bcrypt.hash(password, BCRYPT_COST);
    }

    /**
     * Verifies if the provided password matches the given hash.
     * @param password - The plain text password to verify.
     * @param hash - The hashed password to compare against.
     * @return A promise that resolves to true if the password matches the hash, otherwise false.
     */
    verify(password: string, hash: string): Promise<boolean> {
        return bcrypt.compare(password, hash);
    }
}
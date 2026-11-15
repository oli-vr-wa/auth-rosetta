import { randomUUID } from 'node:crypto';
import {
    BadRequestException,
    ConflictException,
    Injectable,
    UnauthorizedException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { LessThan, QueryFailedError, Repository } from 'typeorm';
import { User } from '@/users/user.entity';
import { PasswordService } from './password.service';
import { TokenService } from './token.service';
import { TotpService } from './totp.service';
import { RevokedToken } from './revoked-token.entity';

const POSTGRES_UNIQUE_VALIDATION = '23505';

@Injectable()
export class AuthService {
    // Used to burn the same time as a real check when the email is unknown.
    private readonly dummyHash: Promise<string>;

    constructor(
        @InjectRepository(User) private readonly users: Repository<User>,
        @InjectRepository(RevokedToken) private readonly revokedTokens: Repository<RevokedToken>,
        private readonly passwords: PasswordService,
        private readonly totp: TotpService,
        private readonly tokens: TokenService,
    ) {
        this.dummyHash = this.passwords.hash(randomUUID());
    }

    /**
     * Normalizes the email address by trimming whitespace and converting it to lowercase.
     * @param email - The email address to normalize.
     * @returns The normalized email address.
     */
    private normalize(email: string): string {
        return email.trim().toLowerCase();
    }

    /**
     * Registers a new user with the provided email and password.
     * @param email - The email address of the new user.
     * @param password - The password for the new user.
     * @throws ConflictException if the email is already registered.
     */
    async register(email: string, password: string) {
        email = this.normalize(email);

        if (await this.users.existsBy({ email })) {
            throw new ConflictException('Email already registered');
        }

        const secret = this.totp.createSecret();
        const user = this.users.create({
            email,
            passwordHash: await this.passwords.hash(password),
            totpSecret: secret
        });

        try {
            await this.users.save(user);
        } catch (err) {
            // Two simultaneous registrations can both pass the check above
            // the unique index catches the loser.
            if (
                err instanceof QueryFailedError &&
                (err.driverError as { code?: string }).code === POSTGRES_UNIQUE_VALIDATION
            ) {
                throw new ConflictException('Email already registered');
            }
            throw err;
        }

        return { otpauthUri: this.totp.buildUri(email, secret), secret };
    }

    /**
     * Confirms the two-factor authentication setup for a user by verifying the provided code.
     * Once confirmed, the user's two-factor authentication is enabled.
     * @param email - The email address of the user.
     * @param code - The TOTP code provided by the user for verification.
     * @throws BadRequestException if the email is unknown or the code is invalid.
     * @returns void
     */
    async confirmTwoFactor(email: string, code: string): Promise<void> {
        const user = await this.users.findOneBy({ email: this.normalize(email) });

        // Unknown email or invalid code should both result in the same error to avoid leaking information about registered emails.
        if (!user || !(await this.totp.verify(user.totpSecret, code))) {
            throw new BadRequestException('Invalid email or code');
        }

        user.twoFactorEnabled = true;
        await this.users.save(user);
    }

    /**
     * Logs in a user by verifying their email, password, and two-factor authentication code.
     * If successful, returns a JWT token for the user.
     * @param email - The email address of the user.
     * @param password - The password provided by the user.
     * @param code - The TOTP code provided by the user for two-factor authentication.
     * @throws UnauthorizedException if the credentials are invalid or two-factor authentication is not enabled.
     * @returns A JWT token for the authenticated user.
     */
    async login(email: string, password: string, code: string): Promise<string> {
        const user = await this.users.findOneBy({ email: this.normalize(email) });

        // Always run a password check, so unknown emails take as long as wrong passwords.
        const passwordOk = await this.passwords.verify(password, user?.passwordHash ?? (await this.dummyHash));

        const codeOk = user ? await this.totp.verify(user.totpSecret, code) : false;

        // Evaluate everything, then fail once with one generic error if anything was wrong. This prevents leaking information about which part was wrong.
        if (!user || !passwordOk || !user.twoFactorEnabled || !codeOk) {
            throw new UnauthorizedException('Invalid credentials');
        }

        return this.tokens.createToken(user.id, user.email);
    }

    /**
     * Logs out a user by revoking their JWT token, preventing its future use.
     * @param jti - The unique identifier of the JWT token to revoke.
     * @param expiresAt - The expiration date of the JWT token to revoke.
     * @returns void
     */
    async logout(jti: string, expiresAt: Date): Promise<void> {
        // Housekeeping: Remove all the expired revoked tokens, as they are no longer needed.
        await this.revokedTokens.delete({ expiresAt: LessThan(new Date()) });

        await this.revokedTokens.upsert({ jti, expiresAt }, ['jti']);
    }

    /**
     * Checks if a JWT token has been revoked.
     * @param jti - The unique identifier of the JWT token to check.
     * @returns A promise that resolves to true if the token is revoked, false otherwise.
     */
    isRevoked(jti: string): Promise<boolean> {
        return this.revokedTokens.existsBy({ jti });
    }
}
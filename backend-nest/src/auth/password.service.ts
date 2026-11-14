import { Injectable } from '@nestjs/common';
import * as bcrypt from 'bcrypt';
import { BCRYPT_COST } from './auth.constants';

@Injectable()
export class PasswordService {
    hash(password: string): Promise<string> {
        return bcrypt.hash(password, BCRYPT_COST);
    }

    verify(password: string, hash: string): Promise<boolean> {
        return bcrypt.compare(password, hash);
    }
}
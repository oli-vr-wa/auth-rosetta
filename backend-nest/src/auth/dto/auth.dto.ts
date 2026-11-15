import { Transform } from 'class-transformer';
import { IsByteLength, IsNotEmpty, IsString, Matches, MaxLength, MinLength } from 'class-validator';
import { Trim } from '@/common/decorators/trim.decorator';

export class RegisterDto {
    @Trim()
    @IsString()
    @IsNotEmpty({ message: 'Email is required' })
    @MaxLength(256, { message: 'Email must be at most 256 characters' })
    email!: string;

    @IsString()
    @MinLength(8, { message: 'Password must be at least 8 characters' })
    @IsByteLength(0, 72, { message: 'Password must be at most 72 characters' })
    password!: string;
}

export class ConfirmDto {
    @IsString()
    email!: string;

    @IsString()
    @Matches(/^\d{6}$/, { message: 'Code must be a 6-digit number' })
    code!: string;
}

export class LoginDto {
    @IsString()
    email!: string;

    @IsString()
    password!: string;

    @IsString()
    code!: string;
}
import { Column, CreateDateColumn, Entity, PrimaryGeneratedColumn } from 'typeorm';

@Entity('users')
export class User {
    @PrimaryGeneratedColumn('uuid')
    id!: string;

    @Column({ length: 256, unique: true, type: 'varchar' })
    email!: string;

    @Column({ name: 'password_hash', type: 'text' })
    passwordHash!: string;

    @Column({ name: 'totp_secret', type: 'text' }) 
    totpSecret!: string;

    @Column({ name: 'two_factor_enabled', type: 'boolean', default: false })
    twoFactorEnabled!: boolean;

    @CreateDateColumn({ name: 'created_at', type: 'timestamptz' })
    createdAt!: Date;
}
import { Column, Entity, PrimaryColumn } from 'typeorm';

@Entity('revoked_tokens')
export class RevokedToken {
    @PrimaryColumn({ type: 'varchar', length: 64 })
    jti!: string;

    @Column({ name: 'expires_at', type: 'timestamptz' })
    expiresAt!: Date;
}

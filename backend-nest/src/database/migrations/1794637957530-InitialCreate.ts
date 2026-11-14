import { MigrationInterface, QueryRunner } from "typeorm";

export class InitialCreate1794637957530 implements MigrationInterface {
    name = 'InitialCreate1794637957530'

    public async up(queryRunner: QueryRunner): Promise<void> {
        await queryRunner.query(`CREATE TABLE "revoked_tokens" ("jti" character varying(64) NOT NULL, "expires_at" TIMESTAMP WITH TIME ZONE NOT NULL, CONSTRAINT "PK_b18aa48269f87cafba8c6310624" PRIMARY KEY ("jti"))`);
        await queryRunner.query(`CREATE TABLE "users" ("id" uuid NOT NULL DEFAULT uuid_generate_v4(), "email" character varying(256) NOT NULL, "password_hash" text NOT NULL, "totp_secret" text NOT NULL, "two_factor_enabled" boolean NOT NULL DEFAULT false, "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "UQ_97672ac88f789774dd47f7c8be3" UNIQUE ("email"), CONSTRAINT "PK_a3ffb1c0c8416b9fc6f907b7433" PRIMARY KEY ("id"))`);
    }

    public async down(queryRunner: QueryRunner): Promise<void> {
        await queryRunner.query(`DROP TABLE "users"`);
        await queryRunner.query(`DROP TABLE "revoked_tokens"`);
    }

}

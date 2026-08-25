"""add device telemetry policy

Revision ID: d4e5f6a7b8c9
Revises: 91c5e8a3f204
Create Date: 2026-08-24 22:15:00.000000
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "d4e5f6a7b8c9"
down_revision: str | Sequence[str] | None = "91c5e8a3f204"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

policy_type = sa.JSON().with_variant(postgresql.JSONB(astext_type=sa.Text()), "postgresql")
legacy_policy = {
    "sampling_seconds": 5,
    "upload_seconds": 30,
    "parked_sampling_seconds": 900,
    "parked_upload_seconds": 900,
}
default_policy = (
    '{"sampling_seconds":120,"upload_seconds":120,'
    '"parked_sampling_seconds":900,"parked_upload_seconds":900}'
)


def _add_policy(table_name: str) -> None:
    with op.batch_alter_table(table_name) as batch_op:
        batch_op.add_column(sa.Column("telemetry_policy", policy_type, nullable=True))
    policy_table = sa.table(table_name, sa.column("telemetry_policy", policy_type))
    op.execute(policy_table.update().values(telemetry_policy=legacy_policy))
    with op.batch_alter_table(table_name) as batch_op:
        batch_op.alter_column(
            "telemetry_policy",
            existing_type=policy_type,
            nullable=False,
            server_default=default_policy,
        )


def upgrade() -> None:
    _add_policy("devices")
    _add_policy("device_enrollment_tokens")


def downgrade() -> None:
    with op.batch_alter_table("device_enrollment_tokens") as batch_op:
        batch_op.drop_column("telemetry_policy")
    with op.batch_alter_table("devices") as batch_op:
        batch_op.drop_column("telemetry_policy")

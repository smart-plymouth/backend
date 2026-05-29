"""Create planning_cases table

Revision ID: 003
Revises: 002
Create Date: 2026-05-29

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "003"
down_revision: Union[str, None] = "002"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "planning_cases",
        sa.Column("reference", sa.String(50), primary_key=True),
        sa.Column("address", sa.Text, nullable=False),
        sa.Column("proposal", sa.Text, nullable=False),
        sa.Column("status", sa.String(100), nullable=False),
        sa.Column("received_date", sa.Date, nullable=True),
        sa.Column("validated_date", sa.Date, nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
    )

    # Index on validated_date for efficient weekly queries
    op.create_index(
        "ix_planning_cases_validated_date",
        "planning_cases",
        ["validated_date"],
    )


def downgrade() -> None:
    op.drop_index("ix_planning_cases_validated_date", table_name="planning_cases")
    op.drop_table("planning_cases")

"""Create planning_supports table

Revision ID: 011
Revises: 010
Create Date: 2026-06-30

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "011"
down_revision: Union[str, None] = "010"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "planning_supports",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column(
            "case_reference",
            sa.String(50),
            sa.ForeignKey("planning_cases.reference"),
            nullable=False,
            index=True,
        ),
        sa.Column("support_reason", sa.UnicodeText, nullable=False),
        sa.Column("ai_rationalisation", sa.UnicodeText, nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
    )


def downgrade() -> None:
    op.drop_table("planning_supports")

"""Create planning_objections table

Revision ID: 006
Revises: 005
Create Date: 2026-05-29

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "006"
down_revision: Union[str, None] = "005"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "planning_objections",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column(
            "case_reference",
            sa.String(50),
            sa.ForeignKey("planning_cases.reference"),
            nullable=False,
            index=True,
        ),
        sa.Column("objection", sa.UnicodeText, nullable=False),
        sa.Column("ai_rationalisation", sa.UnicodeText, nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
    )


def downgrade() -> None:
    op.drop_table("planning_objections")

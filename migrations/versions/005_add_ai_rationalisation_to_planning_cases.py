"""Add ai_rationalisation column to planning_cases

Revision ID: 005
Revises: 004
Create Date: 2026-05-29

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "005"
down_revision: Union[str, None] = "004"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column(
        "planning_cases",
        sa.Column("ai_rationalisation", sa.UnicodeText, nullable=True),
    )


def downgrade() -> None:
    op.drop_column("planning_cases", "ai_rationalisation")

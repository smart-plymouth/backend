"""Add pros and cons columns to planning_cases

Revision ID: 007
Revises: 006
Create Date: 2026-06-05

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "007"
down_revision: Union[str, None] = "006"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column(
        "planning_cases",
        sa.Column("pros", sa.JSON, nullable=True),
    )
    op.add_column(
        "planning_cases",
        sa.Column("cons", sa.JSON, nullable=True),
    )


def downgrade() -> None:
    op.drop_column("planning_cases", "cons")
    op.drop_column("planning_cases", "pros")

"""Add ai_analysis, potential_impact_score, tags, estimated_size to planning_cases

Revision ID: 004
Revises: 003
Create Date: 2026-05-29

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "004"
down_revision: Union[str, None] = "003"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column(
        "planning_cases",
        sa.Column("ai_analysis", sa.Boolean, nullable=False, server_default="false"),
    )
    op.add_column(
        "planning_cases",
        sa.Column("potential_impact_score", sa.Integer, nullable=True),
    )
    op.add_column(
        "planning_cases",
        sa.Column("tags", sa.JSON, nullable=True),
    )
    op.add_column(
        "planning_cases",
        sa.Column("estimated_size", sa.Integer, nullable=True),
    )


def downgrade() -> None:
    op.drop_column("planning_cases", "estimated_size")
    op.drop_column("planning_cases", "tags")
    op.drop_column("planning_cases", "potential_impact_score")
    op.drop_column("planning_cases", "ai_analysis")

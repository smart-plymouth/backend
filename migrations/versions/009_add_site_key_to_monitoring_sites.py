"""Add site_key column to monitoring_sites

Revision ID: 009
Revises: 008
Create Date: 2026-06-06

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "009"
down_revision: Union[str, None] = "008"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column(
        "monitoring_sites",
        sa.Column("site_key", sa.String(255), nullable=True),
    )
    op.create_index(
        "ix_monitoring_sites_site_key",
        "monitoring_sites",
        ["site_key"],
        unique=True,
    )


def downgrade() -> None:
    op.drop_index("ix_monitoring_sites_site_key", table_name="monitoring_sites")
    op.drop_column("monitoring_sites", "site_key")

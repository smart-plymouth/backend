"""Create bird monitoring tables (monitoring_sites, species, species_sightings)

Revision ID: 008
Revises: 007
Create Date: 2026-06-06

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "008"
down_revision: Union[str, None] = "007"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "monitoring_sites",
        sa.Column("site_id", sa.Uuid, primary_key=True),
        sa.Column("name", sa.String(255), nullable=False),
        sa.Column("latitude", sa.Float, nullable=False),
        sa.Column("longitude", sa.Float, nullable=False),
        sa.Column("type", sa.String(50), nullable=False, server_default="BirdNET-Pi"),
    )

    op.create_table(
        "species",
        sa.Column("species_id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("common_name", sa.String(255), nullable=False, unique=True),
    )

    op.create_table(
        "species_sightings",
        sa.Column("sighting_id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column(
            "site_id",
            sa.Uuid,
            sa.ForeignKey("monitoring_sites.site_id"),
            nullable=False,
        ),
        sa.Column(
            "species_id",
            sa.Integer,
            sa.ForeignKey("species.species_id"),
            nullable=False,
        ),
        sa.Column("confidence", sa.Float, nullable=False),
        sa.Column(
            "datetime",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
    )

    op.create_index(
        "ix_species_sightings_site_id",
        "species_sightings",
        ["site_id"],
    )
    op.create_index(
        "ix_species_sightings_species_id",
        "species_sightings",
        ["species_id"],
    )
    op.create_index(
        "ix_species_sightings_datetime",
        "species_sightings",
        ["datetime"],
    )


def downgrade() -> None:
    op.drop_index("ix_species_sightings_datetime", table_name="species_sightings")
    op.drop_index("ix_species_sightings_species_id", table_name="species_sightings")
    op.drop_index("ix_species_sightings_site_id", table_name="species_sightings")
    op.drop_table("species_sightings")
    op.drop_table("species")
    op.drop_table("monitoring_sites")

"""Create ed_wait_times tables

Revision ID: 001
Revises: None
Create Date: 2025-05-28

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects.postgresql import UUID

# revision identifiers, used by Alembic.
revision: str = "001"
down_revision: Union[str, None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Create location_type enum
    location_type_enum = sa.Enum(
        "emergency_department",
        "urgent_treatment_centre",
        "minor_injuries_unit",
        name="location_type",
    )
    location_type_enum.create(op.get_bind(), checkfirst=True)

    # Create locations table
    op.create_table(
        "ed_wait_times_locations",
        sa.Column("id", UUID(as_uuid=True), primary_key=True),
        sa.Column("name", sa.String(255), nullable=False),
        sa.Column("type", location_type_enum, nullable=False),
        sa.Column("address", sa.Text, nullable=False),
        sa.Column("longitude", sa.Float, nullable=False),
        sa.Column("latitude", sa.Float, nullable=False),
        sa.Column("opening_times", sa.Text, nullable=True),
        sa.Column("telephone_number", sa.String(50), nullable=True),
    )

    # Create wait times table with composite primary key
    op.create_table(
        "ed_wait_times_wait_times",
        sa.Column("location_id", UUID(as_uuid=True), sa.ForeignKey("ed_wait_times_locations.id"), primary_key=True),
        sa.Column("timestamp", sa.DateTime(timezone=True), primary_key=True),
        sa.Column("longest_wait", sa.Integer, nullable=False),
        sa.Column("patients_waiting", sa.Integer, nullable=False),
        sa.Column("patients_in_department", sa.Integer, nullable=False),
    )


def downgrade() -> None:
    op.drop_table("ed_wait_times_wait_times")
    op.drop_table("ed_wait_times_locations")
    sa.Enum(name="location_type").drop(op.get_bind(), checkfirst=True)

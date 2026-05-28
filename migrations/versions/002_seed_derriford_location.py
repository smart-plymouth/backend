"""Seed Derriford ED and UTC Dartmoor locations

Revision ID: 002
Revises: 001
Create Date: 2025-05-28

"""
from typing import Sequence, Union

from alembic import op

# revision identifiers, used by Alembic.
revision: str = "002"
down_revision: Union[str, None] = "001"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None

DERRIFORD_ID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
UTC_DARTMOOR_ID = "b2c3d4e5-f6a7-8901-bcde-f12345678901"
UTC_CUMBERLAND_ID = "c3d4e5f6-a7b8-9012-cdef-123456789012"
MIU_TAVISTOCK_ID = "d4e5f6a7-b8c9-0123-defa-234567890123"
MIU_KINGSBRIDGE_ID = "e5f6a7b8-c9d0-1234-efab-345678901234"


def upgrade() -> None:
    op.execute(f"""
        INSERT INTO ed_wait_times_locations (id, name, type, address, longitude, latitude, opening_times, telephone_number)
        VALUES (
            '{DERRIFORD_ID}',
            'Emergency Department',
            'emergency_department',
            'Derriford Hospital, Derriford Rd, Crownhill, Plymouth PL6 8DH',
            50.41711142234749,
            -4.113110116204163,
            '24/7',
            '01752202082'
        )
        ON CONFLICT (id) DO NOTHING;
    """)

    op.execute(f"""
        INSERT INTO ed_wait_times_locations (id, name, type, address, longitude, latitude, opening_times, telephone_number)
        VALUES (
            '{UTC_DARTMOOR_ID}',
            'UTC Dartmoor',
            'urgent_treatment_centre',
            'Dartmoor Building, Derriford Hospital, Derriford Rd, Crownhill, Plymouth PL6 8DH',
            50.41777405343772,
            -4.118717898460078,
            '8am to 8pm 7 days a week',
            '01752438320'
        )
        ON CONFLICT (id) DO NOTHING;
    """)

    op.execute(f"""
        INSERT INTO ed_wait_times_locations (id, name, type, address, longitude, latitude, opening_times, telephone_number)
        VALUES (
            '{UTC_CUMBERLAND_ID}',
            'UTC Cumberland Centre',
            'urgent_treatment_centre',
            'Cumberland Centre, Damerel Cl, Devonport, Plymouth PL1 4TZ',
            50.37004786828604,
            -4.168900819746478,
            '8am to 8pm 7 days a week',
            '01752434390'
        )
        ON CONFLICT (id) DO NOTHING;
    """)

    op.execute(f"""
        INSERT INTO ed_wait_times_locations (id, name, type, address, longitude, latitude, opening_times, telephone_number)
        VALUES (
            '{MIU_TAVISTOCK_ID}',
            'MIU Tavistock',
            'minor_injuries_unit',
            'Minor Injury Unit, Tavistock Hospital, Spring Hill, Tavistock, PL19 8LD',
            50.54749012841367,
            -4.15343141841533,
            '8:30am to 5.30pm 7 days per week',
            '01822612233'
        )
        ON CONFLICT (id) DO NOTHING;
    """)

    op.execute(f"""
        INSERT INTO ed_wait_times_locations (id, name, type, address, longitude, latitude, opening_times, telephone_number)
        VALUES (
            '{MIU_KINGSBRIDGE_ID}',
            'MIU Kingsbridge',
            'minor_injuries_unit',
            'Minor Injury Unit, South Hams Hospital, Plymouth Road, Kingsbridge, Devon, TQ7 1AT',
            50.28936430309413,
            -3.7813150917241485,
            '8:30am to 5.30pm 7 days per week',
            '01548852349'
        )
        ON CONFLICT (id) DO NOTHING;
    """)


def downgrade() -> None:
    op.execute(f"""
        DELETE FROM ed_wait_times_locations WHERE id IN ('{DERRIFORD_ID}', '{UTC_DARTMOOR_ID}', '{UTC_CUMBERLAND_ID}', '{MIU_TAVISTOCK_ID}', '{MIU_KINGSBRIDGE_ID}');
    """)

import uuid

from app import db
from sqlalchemy.dialects.postgresql import UUID


class Location(db.Model):
    __tablename__ = "ed_wait_times_locations"

    id = db.Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    name = db.Column(db.String(255), nullable=False)
    type = db.Column(
        db.Enum(
            "emergency_department",
            "urgent_treatment_centre",
            "minor_injuries_unit",
            name="location_type",
        ),
        nullable=False,
    )
    address = db.Column(db.Text, nullable=False)
    longitude = db.Column(db.Float, nullable=False)
    latitude = db.Column(db.Float, nullable=False)
    opening_times = db.Column(db.Text, nullable=True)
    telephone_number = db.Column(db.String(50), nullable=True)

    wait_times = db.relationship("WaitTime", back_populates="location", lazy="dynamic")

    def to_dict(self):
        return {
            "id": str(self.id),
            "name": self.name,
            "type": self.type,
            "address": self.address,
            "longitude": self.longitude,
            "latitude": self.latitude,
            "opening_times": self.opening_times,
            "telephone_number": self.telephone_number,
        }


class WaitTime(db.Model):
    __tablename__ = "ed_wait_times_wait_times"

    location_id = db.Column(
        UUID(as_uuid=True),
        db.ForeignKey("ed_wait_times_locations.id"),
        primary_key=True,
    )
    timestamp = db.Column(db.DateTime(timezone=True), primary_key=True)
    longest_wait = db.Column(db.Integer, nullable=False)
    patients_waiting = db.Column(db.Integer, nullable=False)
    patients_in_department = db.Column(db.Integer, nullable=False)

    location = db.relationship("Location", back_populates="wait_times")

    def to_dict(self):
        return {
            "location_id": str(self.location_id),
            "timestamp": self.timestamp.isoformat(),
            "longest_wait": self.longest_wait,
            "patients_waiting": self.patients_waiting,
            "patients_in_department": self.patients_in_department,
        }

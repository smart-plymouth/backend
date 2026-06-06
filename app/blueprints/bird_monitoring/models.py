import uuid

from app import db


class MonitoringSite(db.Model):
    __tablename__ = "monitoring_sites"

    site_id = db.Column(
        db.Uuid, primary_key=True, default=uuid.uuid4
    )
    name = db.Column(db.String(255), nullable=False)
    latitude = db.Column(db.Float, nullable=False)
    longitude = db.Column(db.Float, nullable=False)
    type = db.Column(db.String(50), nullable=False, default="BirdNET-Pi")
    site_key = db.Column(db.String(255), nullable=True, unique=True)

    sightings = db.relationship(
        "SpeciesSighting", back_populates="site", lazy="dynamic"
    )

    def to_dict(self):
        return {
            "site_id": str(self.site_id),
            "name": self.name,
            "latitude": self.latitude,
            "longitude": self.longitude,
            "type": self.type,
        }


class Species(db.Model):
    __tablename__ = "species"

    species_id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    common_name = db.Column(db.String(255), nullable=False, unique=True)

    sightings = db.relationship(
        "SpeciesSighting", back_populates="species_rel", lazy="dynamic"
    )

    def to_dict(self):
        return {
            "species_id": self.species_id,
            "common_name": self.common_name,
        }


class SpeciesSighting(db.Model):
    __tablename__ = "species_sightings"

    sighting_id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    site_id = db.Column(
        db.Uuid,
        db.ForeignKey("monitoring_sites.site_id"),
        nullable=False,
        index=True,
    )
    species_id = db.Column(
        db.Integer,
        db.ForeignKey("species.species_id"),
        nullable=False,
        index=True,
    )
    confidence = db.Column(db.Float, nullable=False)
    datetime = db.Column(
        db.DateTime(timezone=True),
        server_default=db.func.now(),
        nullable=False,
    )

    site = db.relationship("MonitoringSite", back_populates="sightings")
    species_rel = db.relationship("Species", back_populates="sightings")

    def to_dict(self):
        return {
            "sighting_id": self.sighting_id,
            "site_id": str(self.site_id),
            "species": self.species_rel.to_dict() if self.species_rel else None,
            "confidence": self.confidence,
            "datetime": self.datetime.isoformat() if self.datetime else None,
        }

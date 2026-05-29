from app import db


class PlanningCase(db.Model):
    __tablename__ = "planning_cases"

    reference = db.Column(db.String(50), primary_key=True)
    address = db.Column(db.Text, nullable=False)
    proposal = db.Column(db.Text, nullable=False)
    status = db.Column(db.String(100), nullable=False)
    received_date = db.Column(db.Date, nullable=True)
    validated_date = db.Column(db.Date, nullable=True)
    ai_analysis = db.Column(db.Boolean, nullable=False, default=False, server_default="false")
    potential_impact_score = db.Column(db.Integer, nullable=True)
    tags = db.Column(db.JSON, nullable=True)
    estimated_size = db.Column(db.Integer, nullable=True)
    ai_rationalisation = db.Column(db.UnicodeText, nullable=True)
    created_at = db.Column(
        db.DateTime(timezone=True), server_default=db.func.now(), nullable=False
    )
    updated_at = db.Column(
        db.DateTime(timezone=True),
        server_default=db.func.now(),
        onupdate=db.func.now(),
        nullable=False,
    )

    objections = db.relationship(
        "PlanningObjection", back_populates="case", lazy="dynamic"
    )

    def to_dict(self):
        return {
            "reference": self.reference,
            "address": self.address,
            "proposal": self.proposal,
            "status": self.status,
            "received_date": (
                self.received_date.isoformat() if self.received_date else None
            ),
            "validated_date": (
                self.validated_date.isoformat() if self.validated_date else None
            ),
            "ai_analysis": self.ai_analysis,
            "potential_impact_score": self.potential_impact_score,
            "tags": self.tags,
            "estimated_size": self.estimated_size,
            "ai_rationalisation": self.ai_rationalisation,
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None,
        }


class PlanningObjection(db.Model):
    __tablename__ = "planning_objections"

    id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    case_reference = db.Column(
        db.String(50),
        db.ForeignKey("planning_cases.reference"),
        nullable=False,
        index=True,
    )
    objection = db.Column(db.UnicodeText, nullable=False)
    ai_rationalisation = db.Column(db.UnicodeText, nullable=False)
    created_at = db.Column(
        db.DateTime(timezone=True), server_default=db.func.now(), nullable=False
    )

    case = db.relationship("PlanningCase", back_populates="objections")

    def to_dict(self):
        return {
            "id": self.id,
            "case_reference": self.case_reference,
            "objection": self.objection,
            "ai_rationalisation": self.ai_rationalisation,
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }

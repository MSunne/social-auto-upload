from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.db.session import get_db
from app.models.product_skill import ProductSkill
from app.models.user import User
from app.schemas.skill import ProductSkillCreateRequest, ProductSkillResponse, ProductSkillUpdateRequest


router = APIRouter()


@router.get("", response_model=list[ProductSkillResponse])
def list_skills(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    statement = (
        select(ProductSkill)
        .where(ProductSkill.owner_user_id == current_user.id)
        .order_by(ProductSkill.updated_at.desc())
    )
    return list(db.scalars(statement))


@router.post("", response_model=ProductSkillResponse, status_code=status.HTTP_201_CREATED)
def create_skill(
    payload: ProductSkillCreateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    skill = ProductSkill(owner_user_id=current_user.id, **payload.model_dump())
    db.add(skill)
    db.commit()
    db.refresh(skill)
    return skill


@router.patch("/{skill_id}", response_model=ProductSkillResponse)
def update_skill(
    skill_id: str,
    payload: ProductSkillUpdateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    skill = db.get(ProductSkill, skill_id)
    if skill is None or skill.owner_user_id != current_user.id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Skill not found")

    for field_name, value in payload.model_dump(exclude_unset=True).items():
        setattr(skill, field_name, value)

    db.commit()
    db.refresh(skill)
    return skill


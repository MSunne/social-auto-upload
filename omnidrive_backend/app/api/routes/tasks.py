from fastapi import APIRouter, Depends
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.db.session import get_db
from app.models.device import Device
from app.models.publish_task import PublishTask
from app.models.user import User
from app.schemas.task import PublishTaskResponse


router = APIRouter()


@router.get("", response_model=list[PublishTaskResponse])
def list_publish_tasks(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    statement = (
        select(PublishTask)
        .join(Device, PublishTask.device_id == Device.id)
        .where(Device.owner_user_id == current_user.id)
        .order_by(PublishTask.updated_at.desc())
    )
    return list(db.scalars(statement))


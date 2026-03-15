from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.db.session import get_db
from app.models.device import Device
from app.models.login import LoginSession
from app.models.platform_account import PlatformAccount
from app.models.user import User
from app.schemas.account import AccountResponse, RemoteLoginSessionCreateRequest, RemoteLoginSessionResponse


router = APIRouter()


@router.get("", response_model=list[AccountResponse])
def list_accounts(
    device_id: str | None = None,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    statement = (
        select(PlatformAccount)
        .join(Device, PlatformAccount.device_id == Device.id)
        .where(Device.owner_user_id == current_user.id)
        .order_by(PlatformAccount.updated_at.desc())
    )
    if device_id:
        statement = statement.where(PlatformAccount.device_id == device_id)
    return list(db.scalars(statement))


@router.post("/remote-login", response_model=RemoteLoginSessionResponse, status_code=status.HTTP_201_CREATED)
def create_remote_login_session(
    payload: RemoteLoginSessionCreateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    device = db.get(Device, payload.device_id)
    if device is None or device.owner_user_id != current_user.id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Device not found")

    session = LoginSession(
        device_id=device.id,
        user_id=current_user.id,
        platform=payload.platform,
        account_name=payload.account_name,
        status="pending",
        message="等待本地 OmniBull 拉起登录流程",
    )
    db.add(session)
    db.commit()
    db.refresh(session)
    return session


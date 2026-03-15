from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.db.session import get_db
from app.models.device import Device
from app.models.user import User
from app.schemas.device import DeviceClaimRequest, DeviceResponse, DeviceUpdateRequest


router = APIRouter()


@router.get("", response_model=list[DeviceResponse])
def list_devices(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    statement = (
        select(Device)
        .where(Device.owner_user_id == current_user.id)
        .order_by(Device.updated_at.desc())
    )
    return list(db.scalars(statement))


@router.post("/claim", response_model=DeviceResponse)
def claim_device(
    payload: DeviceClaimRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    device = db.scalar(select(Device).where(Device.device_code == payload.device_code))
    if device is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Device code not found")
    if device.owner_user_id and device.owner_user_id != current_user.id:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="Device already claimed")

    device.owner_user_id = current_user.id
    device.is_enabled = True
    db.commit()
    db.refresh(device)
    return device


@router.patch("/{device_id}", response_model=DeviceResponse)
def update_device(
    device_id: str,
    payload: DeviceUpdateRequest,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    device = db.get(Device, device_id)
    if device is None or device.owner_user_id != current_user.id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Device not found")

    if payload.name is not None:
        device.name = payload.name
    if payload.default_reasoning_model is not None:
        device.default_reasoning_model = payload.default_reasoning_model
    if payload.is_enabled is not None:
        device.is_enabled = payload.is_enabled

    db.commit()
    db.refresh(device)
    return device


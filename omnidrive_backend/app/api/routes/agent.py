from datetime import datetime

from fastapi import APIRouter, Depends, Header, HTTPException, status
from sqlalchemy import select
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models.device import Device
from app.models.login import LoginSession
from app.schemas.agent import (
    AgentHeartbeatRequest,
    AgentHeartbeatResponse,
    AgentLoginSessionEventRequest,
    AgentLoginTaskResponse,
)


router = APIRouter()


def resolve_agent_device(db: Session, device_code: str, agent_key: str) -> Device:
    device = db.scalar(select(Device).where(Device.device_code == device_code))
    if device is None:
        device = Device(
            device_code=device_code,
            agent_key=agent_key,
            name=device_code,
            is_enabled=False,
        )
        db.add(device)
        db.commit()
        db.refresh(device)
        return device

    if device.agent_key and device.agent_key != agent_key:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Agent key mismatch")
    if not device.agent_key:
        device.agent_key = agent_key
        db.commit()
        db.refresh(device)
    return device


@router.post("/heartbeat", response_model=AgentHeartbeatResponse)
def heartbeat(payload: AgentHeartbeatRequest, db: Session = Depends(get_db)):
    device = resolve_agent_device(db, payload.device_code, payload.agent_key)
    device.name = payload.device_name or device.name
    device.local_ip = payload.local_ip
    device.public_ip = payload.public_ip
    device.runtime_payload = payload.runtime_payload
    device.last_seen_at = datetime.utcnow()
    db.commit()
    db.refresh(device)
    return AgentHeartbeatResponse(device=device)


@router.get("/login-tasks/{device_code}", response_model=list[AgentLoginTaskResponse])
def list_pending_login_tasks(
    device_code: str,
    x_agent_key: str = Header(alias="X-Agent-Key"),
    db: Session = Depends(get_db),
):
    device = resolve_agent_device(db, device_code, x_agent_key)
    statement = (
        select(LoginSession)
        .where(LoginSession.device_id == device.id, LoginSession.status == "pending")
        .order_by(LoginSession.created_at.asc())
    )
    return list(db.scalars(statement))


@router.post("/login-sessions/{session_id}/event")
def push_login_session_event(
    session_id: str,
    payload: AgentLoginSessionEventRequest,
    x_agent_key: str = Header(alias="X-Agent-Key"),
    db: Session = Depends(get_db),
):
    session = db.get(LoginSession, session_id)
    if session is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Session not found")

    device = db.get(Device, session.device_id)
    if device is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Device not found")
    resolve_agent_device(db, device.device_code, x_agent_key)

    session.status = payload.status
    session.message = payload.message
    session.qr_data = payload.qr_data
    session.verification_payload = payload.verification_payload
    db.commit()
    return {"ok": True}


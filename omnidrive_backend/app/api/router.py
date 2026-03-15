from fastapi import APIRouter

from app.api.routes import accounts, agent, auth, devices, skills, tasks


api_router = APIRouter()
api_router.include_router(auth.router, prefix="/auth", tags=["auth"])
api_router.include_router(devices.router, prefix="/devices", tags=["devices"])
api_router.include_router(accounts.router, prefix="/accounts", tags=["accounts"])
api_router.include_router(skills.router, prefix="/skills", tags=["skills"])
api_router.include_router(tasks.router, prefix="/tasks", tags=["tasks"])
api_router.include_router(agent.router, prefix="/agent", tags=["agent"])


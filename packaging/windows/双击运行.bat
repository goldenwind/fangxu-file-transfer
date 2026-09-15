@echo off
chcp 65001 >nul
cd /d "%~dp0"
title 方序传文件
Fangxu-File-Transfer.exe
if errorlevel 1 (
  echo.
  echo 启动失败，请查看上方错误信息。
  pause
)

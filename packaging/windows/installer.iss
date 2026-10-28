; Inno Setup script for mormiPrint. Per-user install (no admin rights), adds a Start menu entry, no startup entry.
; Silent install, as winget runs it:  mormiprint-setup-<version>-windows-x64.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART
; CI replaces __VERSION__ and compiles it (see .github/workflows/build.yml).
[Setup]
AppId={{B3E9A1C4-52D7-4F08-A6E1-9C2D4B7F3A15}
AppName=mormiPrint
AppVersion=__VERSION__
AppPublisher=mormitech
AppPublisherURL=https://github.com/mormitech/mormiprint
AppSupportURL=https://github.com/mormitech/mormiprint/issues
DefaultDirName={autopf}\mormiprint
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableReadyPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\..\dist
OutputBaseFilename=mormiprint-setup-__VERSION__-windows-x64
UninstallDisplayName=mormiPrint
UninstallDisplayIcon={app}\mormiprint.exe
Compression=lzma2
SolidCompression=yes

[Files]
Source: "..\..\build\bin\mormiprint.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\mormiPrint"; Filename: "{app}\mormiprint.exe"

[Run]
Filename: "{app}\mormiprint.exe"; Description: "Start mormiPrint"; Flags: nowait postinstall skipifsilent

[Code]
// taskkill returns before Windows has released the file of the killed process, so wait a moment before the installer
// replaces or deletes it.
procedure StopMormiprint;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM mormiprint.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(1500);
end;

// An upgrade replaces a file that may be in use, so close the running app first.
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then StopMormiprint;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then StopMormiprint;
end;

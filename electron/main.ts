import { app, BrowserWindow, ipcMain, Tray, Menu, dialog, nativeImage } from 'electron'
import * as path from 'path'
import { spawn, ChildProcess } from 'child_process'
import * as fs from 'fs'

const isDev = process.env.NODE_ENV === 'development'

// Enable logging
const logPath = path.join(app.getPath('userData'), 'app.log')
function log(message: string) {
  const timestamp = new Date().toISOString()
  const logMessage = `[${timestamp}] ${message}\n`
  console.log(logMessage)
  fs.appendFileSync(logPath, logMessage)
}

let mainWindow: BrowserWindow | null = null
let tray: Tray | null = null
let backendProcess: ChildProcess | null = null

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 1200,
    minHeight: 700,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js'),
    },
    title: 'NetMonitor - 网络流量监控',
    show: false,
  })

  mainWindow.webContents.on('console-message', (_event, _level, message, line, sourceId) => {
    log(`Console [${sourceId}:${line}]: ${message}`)
  })

  mainWindow.webContents.on('did-fail-load', (_event, errorCode, errorDescription) => {
    log(`Frontend failed to load: ${errorCode} - ${errorDescription}`)
  })

  if (isDev) {
    mainWindow.loadURL('http://localhost:3000')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(path.join(__dirname, '../frontend/index.html'))
  }

  mainWindow.once('ready-to-show', () => {
    mainWindow?.show()
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

app.whenReady().then(() => {
  log('App is ready, initializing...')

  // Try to create tray, but don't fail the app if it fails
  try {
    createTray()
    log('Tray created')
  } catch (trayError: any) {
    log(`Warning: Failed to create tray: ${trayError.message}`)
  }

  // Create window and start backend - these are critical
  try {
    createWindow()
    log('Window created')
    startBackend()
    log('Backend started')
  } catch (error: any) {
    log(`Fatal error: ${error.message}\n${error.stack}`)
    dialog.showErrorBox('NetMonitor 启动失败', error.message)
    app.quit()
  }

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    stopBackend()
    app.quit()
  }
})

app.on('before-quit', () => {
  stopBackend()
})

// IPC handlers
ipcMain.handle('get-app-version', () => {
  return app.getVersion()
})

ipcMain.handle('is-dev', () => {
  return isDev
})

ipcMain.handle('start-capture', async (event, iface: string) => {
  try {
    const response = await fetch('http://localhost:8080/api/v1/capture/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ interface: iface }),
    })
    return await response.json()
  } catch (error: any) {
    return { error: error.message }
  }
})

ipcMain.handle('stop-capture', async () => {
  try {
    const response = await fetch('http://localhost:8080/api/v1/capture/stop', {
      method: 'POST',
    })
    return await response.json()
  } catch (error: any) {
    return { error: error.message }
  }
})

ipcMain.handle('get-interfaces', async () => {
  try {
    const response = await fetch('http://localhost:8080/api/v1/interfaces')
    return await response.json()
  } catch (error: any) {
    return { error: error.message }
  }
})

// Tray menu
function createTray() {
  const iconPath = path.join(__dirname, '../resources/icon.png')
  let trayIcon: Electron.NativeImage

  if (fs.existsSync(iconPath)) {
    trayIcon = nativeImage.createFromPath(iconPath)
  } else {
    log(`Tray icon not found at ${iconPath}, using empty icon as fallback`)
    trayIcon = nativeImage.createEmpty()
  }

  tray = new Tray(trayIcon)

  const contextMenu = Menu.buildFromTemplate([
    {
      label: '打开 NetMonitor',
      click: () => {
        if (mainWindow) {
          mainWindow.show()
          mainWindow.focus()
        }
      },
    },
    {
      label: '开始监控',
      click: () => {
        mainWindow?.webContents.send('start-capture')
      },
    },
    {
      label: '停止监控',
      click: () => {
        mainWindow?.webContents.send('stop-capture')
      },
    },
    { type: 'separator' },
    {
      label: '退出',
      click: () => {
        app.quit()
      },
    },
  ])

  tray.setToolTip('NetMonitor - 网络流量监控')
  tray.setContextMenu(contextMenu)

  tray.on('click', () => {
    if (mainWindow) {
      mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show()
    }
  })
}

// Backend process management
function startBackend() {
  const backendPath = isDev
    ? path.join(__dirname, '../../resources/backend-server.exe')
    : path.join(process.resourcesPath, 'backend-server.exe')

  backendProcess = spawn(backendPath, [], {
    stdio: ['pipe', 'pipe', 'pipe'],
    windowsHide: true,
  })

  backendProcess.stdout?.on('data', (data: Buffer) => {
    log(`[backend] ${data.toString().trim()}`)
  })

  backendProcess.stderr?.on('data', (data: Buffer) => {
    log(`[backend-err] ${data.toString().trim()}`)
  })

  backendProcess.on('error', (error) => {
    log(`Failed to start backend: ${error.message}`)
  })

  backendProcess.on('exit', (code: number | null) => {
    log(`Backend process exited with code ${code}`)
    if (code !== 0 && code !== null) {
      log(`ERROR: Backend exited abnormally`)
    }
  })
}

function stopBackend() {
  if (backendProcess) {
    backendProcess.kill()
    backendProcess = null
  }
}

export class WebSocketClient {
  private ws: WebSocket | null = null
  private url: string
  private reconnectInterval: number = 3000
  private maxReconnectAttempts: number = 5
  private reconnectAttempts: number = 0
  private onMessageCallbacks: Map<string, Function[]> = new Map()
  private isManualClose: boolean = false

  constructor(url: string = 'ws://localhost:8080/ws/realtime') {
    this.url = url
  }

  connect() {
    this.isManualClose = false
    this.connectInternal()
  }

  private connectInternal() {
    try {
      this.ws = new WebSocket(this.url)

      this.ws.onopen = () => {
        console.log('WebSocket connected')
        this.reconnectAttempts = 0
      }

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          this.handleMessage(data)
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }

      this.ws.onclose = () => {
        console.log('WebSocket closed')
        if (!this.isManualClose && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++
          console.log(`Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
          setTimeout(() => this.connectInternal(), this.reconnectInterval)
        }
      }
    } catch (error) {
      console.error('Failed to connect WebSocket:', error)
    }
  }

  disconnect() {
    this.isManualClose = true
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  on(type: string, callback: Function) {
    if (!this.onMessageCallbacks.has(type)) {
      this.onMessageCallbacks.set(type, [])
    }
    this.onMessageCallbacks.get(type)!.push(callback)
  }

  off(type: string, callback: Function) {
    const callbacks = this.onMessageCallbacks.get(type)
    if (callbacks) {
      const index = callbacks.indexOf(callback)
      if (index > -1) {
        callbacks.splice(index, 1)
      }
    }
  }

  private handleMessage(data: any) {
    const { type, data: payload, timestamp } = data
    const callbacks = this.onMessageCallbacks.get(type)
    if (callbacks) {
      callbacks.forEach(callback => callback(payload, timestamp))
    }
  }

  send(data: any) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    }
  }
}

export default WebSocketClient

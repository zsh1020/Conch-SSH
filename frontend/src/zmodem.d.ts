declare module 'zmodem.js' {
  export interface ZFileDetails {
    name: string
    size: number | null
    mtime: Date | null
    mode: number | null
    files_remaining: number | null
    bytes_remaining: number | null
  }

  export interface ZTransfer {
    send(data: Uint8Array | number[]): void
    end(data?: Uint8Array | number[]): Promise<void>
    get_details(): ZFileDetails
    get_offset(): number
  }

  export interface ZOffer {
    get_details(): ZFileDetails
    get_offset(): number
    accept(opts?: {
      on_input?: (payload: number[]) => void
      offset?: number
    }): Promise<Uint8Array[] | undefined>
    skip(): void
  }

  export interface ZSession {
    type: 'send' | 'receive'
    on(event: 'offer', cb: (offer: ZOffer) => void): void
    start(): Promise<void>
    close(): Promise<void>
    abort(): void
    aborted(): boolean
    send_offer(params: {
      name: string
      size: number
      mtime?: Date
      files_remaining?: number
      bytes_remaining?: number
    }): Promise<ZTransfer | undefined>
  }

  export interface ZDetection {
    confirm(): ZSession
    deny(): void
  }

  export interface ZSentryOptions {
    to_terminal(octets: number[]): void
    sender(octets: number[]): void
    on_retract(): void
    on_detect(detection: ZDetection): void
  }

  export class Sentry {
    constructor(options: ZSentryOptions)
    consume(data: number[]): void
  }
}

export const ADMIN_MEDIA_MAX_SOURCE_BYTES = 2 * 1024 * 1024
export const ADMIN_MEDIA_MAX_ENCODED_BYTES = 1024 * 1024
export const ADMIN_MEDIA_TYPES = ['image/jpeg', 'image/png', 'image/webp']

function encodeImage(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const objectURL = URL.createObjectURL(file)
    const image = new Image()
    image.onload = () => {
      URL.revokeObjectURL(objectURL)
      const scale = Math.min(1, 1600 / Math.max(image.naturalWidth, image.naturalHeight))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
      canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
      const context = canvas.getContext('2d')
      if (!context) {
        reject(new Error('canvas unavailable'))
        return
      }
      context.fillStyle = '#ffffff'
      context.fillRect(0, 0, canvas.width, canvas.height)
      context.drawImage(image, 0, 0, canvas.width, canvas.height)
      const qualities = [0.88, 0.78, 0.68, 0.58]
      const tryEncode = (index: number) => {
        canvas.toBlob((blob) => {
          if (!blob) {
            reject(new Error('encode failed'))
            return
          }
          if (blob.size > ADMIN_MEDIA_MAX_ENCODED_BYTES && index + 1 < qualities.length) {
            tryEncode(index + 1)
            return
          }
          if (blob.size > ADMIN_MEDIA_MAX_ENCODED_BYTES) {
            reject(new Error('encoded image too large'))
            return
          }
          const reader = new FileReader()
          reader.onload = () => {
            const data = String(reader.result).split(',')[1] ?? ''
            if (!data) reject(new Error('empty image'))
            else resolve(data)
          }
          reader.onerror = () => reject(new Error('read failed'))
          reader.readAsDataURL(blob)
        }, 'image/jpeg', qualities[index])
      }
      tryEncode(0)
    }
    image.onerror = () => {
      URL.revokeObjectURL(objectURL)
      reject(new Error('not an image'))
    }
    image.src = objectURL
  })
}

export function adminMediaError(err: unknown): string {
  const status = (err as { response?: { status?: number }, status?: number } | null)?.response?.status
    ?? (err as { status?: number } | null)?.status
  if (status === 400) return '图片不符合要求：仅支持 jpg/png/webp，且不超过 2MB'
  if (status === 401 || status === 403) return '当前账号没有上传权限'
  return '图片上传失败，请稍后重试'
}

export function useAdminMedia() {
  const { authFetch } = useAuth()

  async function uploadImage(file: File): Promise<string> {
    if (!ADMIN_MEDIA_TYPES.includes(file.type)) throw new Error('unsupported image type')
    if (file.size > ADMIN_MEDIA_MAX_SOURCE_BYTES) throw new Error('source image too large')
    const data = await encodeImage(file)
    const response = await authFetch<{ url?: string }>('/api/v1/admin/media/images', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { data },
    })
    if (!response?.url || !/^\/v1\/assets\/media\/[0-9a-f]{32}\.(?:jpg|png|webp)$/.test(response.url))
      throw new Error('invalid media response')
    return response.url
  }

  return { uploadImage }
}

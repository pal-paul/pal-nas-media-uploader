import type { ImgHTMLAttributes, VideoHTMLAttributes } from 'react'

type ImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, 'src'> & { src: string }

export function AuthenticatedImage({ src, ...props }: ImageProps) {
  return <img src={src} loading="lazy" {...props} />
}

type VideoProps = Omit<VideoHTMLAttributes<HTMLVideoElement>, 'src'> & { src: string }

export function AuthenticatedVideo({ src, ...props }: VideoProps) {
  return <video src={src} {...props} />
}

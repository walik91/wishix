import { t } from './i18n.js'
import headphones from './assets/examples/wish-headphones.png'
import camera from './assets/examples/wish-camera.png'

export const exampleWishlistPath = '/wishlist/example'

export function exampleWishlist() {
  return {
    list: {
      title: t('exampleWishlist.title'),
      description: t('exampleWishlist.description'),
    },
    wishes: [
      {
        id: 'example-headphones',
        title: t('exampleWishlist.headphones'),
        image_url: headphones,
        price: 120,
        currency: 'EUR',
        priority: 'high',
        shop_links: [],
      },
      {
        id: 'example-camera',
        title: t('exampleWishlist.camera'),
        image_url: camera,
        price: 350,
        currency: 'EUR',
        priority: 'medium',
        reserved: true,
        shop_links: [],
      },
    ],
  }
}

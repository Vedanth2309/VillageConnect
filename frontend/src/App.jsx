import { useEffect, useMemo, useState } from 'react'
import axios from 'axios'
import {
  ArrowRight,
  CheckCircle2,
  ChevronRight,
  HeartHandshake,
  Leaf,
  MapPin,
  Search,
  ShieldCheck,
  ShoppingCart,
  Star,
  Truck,
  Users,
} from 'lucide-react'
import './App.css'

const fallbackVillages = [
  { id: 'v1', name: 'Kudlu', district: 'Udupi', state: 'Karnataka' },
  { id: 'v2', name: 'Anekal', district: 'Bengaluru Rural', state: 'Karnataka' },
  { id: 'v3', name: 'Nandigama', district: 'Krishna', state: 'Andhra Pradesh' },
]

const fallbackCategories = ['All', 'Vegetables', 'Fruits', 'Grains', 'Dairy']

const fallbackProducts = [
  {
    id: 'p1',
    name: 'Fresh Tomato',
    category: 'Vegetables',
    price: 38,
    rating: 4.8,
    stock: 22,
    village: 'Kudlu',
    seller: 'Green Valley Farm',
    unit: 'kg',
    available: true,
    description: 'Bright red, locally grown tomatoes for daily cooking and chutneys.',
  },
  {
    id: 'p2',
    name: 'Village Mango',
    category: 'Fruits',
    price: 72,
    rating: 4.9,
    stock: 18,
    village: 'Anekal',
    seller: 'Sunrise Orchard',
    unit: 'kg',
    available: true,
    description: 'Sweet seasonal mangoes naturally ripened in nearby orchards.',
  },
  {
    id: 'p3',
    name: 'Rice Paddy',
    category: 'Grains',
    price: 44,
    rating: 4.6,
    stock: 30,
    village: 'Nandigama',
    seller: 'Riverbank Mills',
    unit: 'kg',
    available: true,
    description: 'Traditional grain cultivated by local farmers with a clean finish.',
  },
  {
    id: 'p4',
    name: 'Farm Eggs',
    category: 'Dairy',
    price: 12,
    rating: 4.7,
    stock: 40,
    village: 'Kudlu',
    seller: 'Happy Hen Co-op',
    unit: 'dozen',
    available: true,
    description: 'Fresh daily eggs from free-range poultry maintained by village families.',
  },
  {
    id: 'p5',
    name: 'Cucumber',
    category: 'Vegetables',
    price: 26,
    rating: 4.5,
    stock: 0,
    village: 'Anekal',
    seller: 'Hill View Greens',
    unit: 'kg',
    available: false,
    description: 'Crisp cucumbers picked fresh for salads and raita preparations.',
  },
]

const formatCurrency = (amount) =>
  new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(amount)

function App() {
  const [villages, setVillages] = useState(fallbackVillages)
  const [categories, setCategories] = useState(fallbackCategories)
  const [products, setProducts] = useState(fallbackProducts)
  const [selectedVillage, setSelectedVillage] = useState(fallbackVillages[0].name)
  const [selectedCategory, setSelectedCategory] = useState('All')
  const [searchText, setSearchText] = useState('')
  const [cart, setCart] = useState([
    { id: 'p1', name: 'Fresh Tomato', price: 38, unit: 'kg', quantity: 1 },
  ])
  const [orderPlaced, setOrderPlaced] = useState(false)

  useEffect(() => {
    const loadData = async () => {
      try {
        const [villagesResponse, categoriesResponse, productsResponse] = await Promise.all([
          axios.get('http://localhost:8080/api/villages'),
          axios.get('http://localhost:8080/api/categories'),
          axios.get('http://localhost:8080/api/products'),
        ])

        if (villagesResponse.data?.villages?.length) {
          setVillages(villagesResponse.data.villages)
          setSelectedVillage(villagesResponse.data.villages[0].name)
        }

        if (categoriesResponse.data?.categories?.length) {
          const categoryNames = ['All', ...categoriesResponse.data.categories.map((c) => c.name)]
          setCategories(categoryNames)
        }

        if (productsResponse.data?.products?.length) {
          setProducts(productsResponse.data.products)
        }
      } catch (error) {
        console.warn('Using fallback data because the API is unavailable:', error)
      }
    }

    loadData()
  }, [])

  const filteredProducts = useMemo(() => {
    return products.filter((product) => {
      const matchesVillage = selectedVillage ? product.village === selectedVillage : true
      const matchesCategory = selectedCategory === 'All' || product.category === selectedCategory
      const matchesSearch =
        !searchText ||
        product.name.toLowerCase().includes(searchText.toLowerCase()) ||
        product.description.toLowerCase().includes(searchText.toLowerCase())

      return matchesVillage && matchesCategory && matchesSearch
    })
  }, [products, selectedVillage, selectedCategory, searchText])

  const cartCount = cart.reduce((sum, item) => sum + item.quantity, 0)
  const subtotal = cart.reduce((sum, item) => sum + item.price * item.quantity, 0)
  const deliveryFee = cart.length ? 25 : 0
  const total = subtotal + deliveryFee

  const addToCart = (product) => {
    setOrderPlaced(false)
    setCart((currentCart) => {
      const existing = currentCart.find((item) => item.id === product.id)
      if (existing) {
        return currentCart.map((item) =>
          item.id === product.id ? { ...item, quantity: item.quantity + 1 } : item,
        )
      }

      return [...currentCart, { id: product.id, name: product.name, price: product.price, unit: product.unit, quantity: 1 }]
    })
  }

  const updateQuantity = (id, delta) => {
    setCart((currentCart) =>
      currentCart.flatMap((item) => {
        if (item.id !== id) return [item]
        const nextQuantity = item.quantity + delta
        return nextQuantity > 0 ? [{ ...item, quantity: nextQuantity }] : []
      }),
    )
  }

  const placeOrder = () => {
    if (!cart.length) return
    setOrderPlaced(true)
    setCart([])
  }

  return (
    <div className="page-shell">
      <header className="topbar">
        <div className="brand-wrap">
          <div className="brand-mark">VC</div>
          <div>
            <div className="brand-name">VillageConnect</div>
            <div className="brand-subtitle">Local marketplace</div>
          </div>
        </div>

        <nav className="nav-links" aria-label="Main navigation">
          <a href="#marketplace">Marketplace</a>
          <a href="#how-it-works">How it works</a>
          <a href="#sellers">Sellers</a>
          <a href="#pricing">Pricing</a>
        </nav>

        <div className="nav-actions">
          <button type="button" className="secondary-button">
            Sign in
          </button>
          <button type="button" className="primary-button">
            Join now
          </button>
        </div>
      </header>

      <main className="main-content">
        <section className="hero-section">
          <div className="hero-copy">
            <span className="eyebrow">
              <Leaf size={16} /> Fresh from local farms
            </span>
            <h1>Village shopping, reimagined for everyday life.</h1>
            <p>
              Discover verified local produce, support nearby sellers, and get farm-fresh goods
              delivered to your village in hours, not days.
            </p>

            <div className="search-bar">
              <Search size={18} />
              <input
                aria-label="Search products"
                value={searchText}
                onChange={(event) => setSearchText(event.target.value)}
                placeholder="Search tomatoes, rice, fruits..."
              />
            </div>

            <div className="hero-meta">
              <div>
                <strong>2-5 villages</strong>
                <span>Launch-ready network</span>
              </div>
              <div>
                <strong>30-50 products</strong>
                <span>Live catalog</span>
              </div>
              <div>
                <strong>Same-day</strong>
                <span>Local delivery</span>
              </div>
            </div>
          </div>

          <div className="hero-panel">
            <div className="panel-header">
              <MapPin size={18} />
              <span>Delivery to</span>
            </div>

            <div className="village-selector">
              {villages.map((village) => (
                <button
                  type="button"
                  key={village.id}
                  className={selectedVillage === village.name ? 'village-chip active' : 'village-chip'}
                  onClick={() => setSelectedVillage(village.name)}
                >
                  {village.name}
                </button>
              ))}
            </div>

            <div className="mini-order-card">
              <div className="mini-order-topline">
                <span>Today’s best picks</span>
                <span className="green-pill">Live</span>
              </div>

              <div className="mini-product-row">
                <div>
                  <strong>Fresh Tomato</strong>
                  <span>Green Valley Farm</span>
                </div>
                <div className="price-tag">₹38/kg</div>
              </div>

              <div className="mini-product-row">
                <div>
                  <strong>Village Mango</strong>
                  <span>Sunrise Orchard</span>
                </div>
                <div className="price-tag">₹72/kg</div>
              </div>

              <button type="button" className="primary-button full-width">
                Shop local now <ArrowRight size={16} />
              </button>
            </div>
          </div>
        </section>

        <section className="category-strip" aria-label="Categories">
          {categories.map((category) => (
            <button
              key={category}
              type="button"
              className={selectedCategory === category ? 'category-pill active' : 'category-pill'}
              onClick={() => setSelectedCategory(category)}
            >
              {category}
            </button>
          ))}
        </section>

        <section className="marketplace-section" id="marketplace">
          <div className="product-column">
            <div className="section-header">
              <div>
                <span className="section-kicker">Marketplace</span>
                <h2>Fresh goods near you</h2>
              </div>
              <span className="result-count">{filteredProducts.length} products</span>
            </div>

            <div className="product-grid">
              {filteredProducts.map((product) => (
                <article key={product.id} className="product-card">
                  <div className="product-visual">
                    <span>{product.category === 'Fruits' ? '🍋' : product.category === 'Grains' ? '🌾' : product.category === 'Dairy' ? '🥚' : '🥬'}</span>
                  </div>

                  <div className="product-body">
                    <div className="product-topline">
                      <span className="tag">{product.category}</span>
                      <span className="rating">
                        <Star size={12} fill="currentColor" /> {product.rating}
                      </span>
                    </div>

                    <h3>{product.name}</h3>
                    <p>{product.description}</p>
                    <div className="product-meta">
                      <span>{product.seller}</span>
                      <span>{product.village}</span>
                    </div>

                    <div className="product-footer">
                      <div>
                        <strong>{formatCurrency(product.price)}</strong>
                        <small>per {product.unit}</small>
                      </div>

                      <button type="button" className="primary-button small" onClick={() => addToCart(product)}>
                        {product.available ? 'Add to cart' : 'Out of stock'}
                      </button>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          </div>

          <aside className="cart-panel">
            <div className="cart-header">
              <div className="cart-title-wrap">
                <ShoppingCart size={18} />
                <h3>Cart</h3>
              </div>
              <span className="cart-badge">{cartCount}</span>
            </div>

            {cart.length === 0 ? (
              <div className="empty-cart">
                <p>Your cart is empty.</p>
                <span>Add fresh items from nearby sellers.</span>
              </div>
            ) : (
              <div className="cart-items">
                {cart.map((item) => (
                  <div key={item.id} className="cart-item">
                    <div>
                      <strong>{item.name}</strong>
                      <span>{formatCurrency(item.price)} / {item.unit}</span>
                    </div>

                    <div className="quantity-controls">
                      <button type="button" onClick={() => updateQuantity(item.id, -1)}>-</button>
                      <span>{item.quantity}</span>
                      <button type="button" onClick={() => updateQuantity(item.id, 1)}>+</button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            <div className="order-summary">
              <div>
                <span>Subtotal</span>
                <strong>{formatCurrency(subtotal)}</strong>
              </div>
              <div>
                <span>Delivery fee</span>
                <strong>{formatCurrency(deliveryFee)}</strong>
              </div>
              <div className="summary-total">
                <span>Total</span>
                <strong>{formatCurrency(total)}</strong>
              </div>
            </div>

            <button type="button" className="checkout-button" onClick={placeOrder} disabled={!cart.length}>
              {orderPlaced ? 'Order placed ✓' : 'Checkout'}
            </button>
          </aside>
        </section>

        <section className="steps-section" id="how-it-works">
          <div className="section-header center-header">
            <div>
              <span className="section-kicker">How it works</span>
              <h2>From farm to village doorstep</h2>
            </div>
          </div>

          <div className="steps-grid">
            <div className="step-card">
              <div className="step-icon green"><MapPin size={18} /></div>
              <h3>Select your village</h3>
              <p>Choose the location you want to support and compare nearby sellers.</p>
            </div>
            <div className="step-card">
              <div className="step-icon green"><Search size={18} /></div>
              <h3>Discover products</h3>
              <p>Use search, categories, and ratings to find fresh household staples.</p>
            </div>
            <div className="step-card">
              <div className="step-icon green"><Truck size={18} /></div>
              <h3>Track the delivery</h3>
              <p>Seller confirms inventory and a village agent handles the last-mile delivery.</p>
            </div>
          </div>
        </section>

        <section className="trust-section" id="sellers">
          <div className="trust-card">
            <div className="trust-copy">
              <span className="section-kicker">Why VillageConnect</span>
              <h2>Built for trust, speed, and local commerce.</h2>
            </div>

            <div className="trust-list">
              <div>
                <ShieldCheck size={18} />
                <span>Verified sellers</span>
              </div>
              <div>
                <HeartHandshake size={18} />
                <span>Community-first pricing</span>
              </div>
              <div>
                <Users size={18} />
                <span>Village-focused logistics</span>
              </div>
              <div>
                <CheckCircle2 size={18} />
                <span>Smarter inventory handling</span>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="footer" id="pricing">
        <div>
          <strong>VillageConnect</strong>
          <span>Fresh food for growing communities.</span>
        </div>
        <div className="footer-links">
          <a href="#marketplace">Marketplace</a>
          <a href="#how-it-works">Process</a>
          <a href="#sellers">Sellers</a>
        </div>
        <button type="button" className="secondary-button">
          Explore nearby sellers <ChevronRight size={16} />
        </button>
      </footer>
    </div>
  )
}

export default App

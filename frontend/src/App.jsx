import { useEffect, useMemo, useState } from 'react'
import axios from 'axios'
import {
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

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:9090'
const PAGE_SIZE = 8

const readSavedUser = () => {
  const token = localStorage.getItem('villageconnect_token')
  const savedUser = localStorage.getItem('villageconnect_user')
  if (!token || !savedUser) return null
  try {
    return JSON.parse(savedUser)
  } catch {
    localStorage.removeItem('villageconnect_token')
    localStorage.removeItem('villageconnect_user')
    return null
  }
}

const getAuthHeaders = () => {
  const token = localStorage.getItem('villageconnect_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

const requestCart = (fulfillmentType = 'delivery') =>
  axios.get(`${API_BASE_URL}/api/cart`, {
    params: { fulfillmentType },
    headers: getAuthHeaders(),
  })

const formatCurrency = (amount) =>
  new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(amount)

function App() {
  const [villages, setVillages] = useState([])
  const [categories, setCategories] = useState([])
  const [products, setProducts] = useState([])
  const [productSuggestions, setProductSuggestions] = useState([])
  const [selectedState, setSelectedState] = useState('')
  const [selectedDistrict, setSelectedDistrict] = useState('')
  const [selectedTaluk, setSelectedTaluk] = useState('')
  const [selectedVillageId, setSelectedVillageId] = useState('')
  const [selectedCategory, setSelectedCategory] = useState('')
  const [searchText, setSearchText] = useState('')
  const [minPrice, setMinPrice] = useState('')
  const [maxPrice, setMaxPrice] = useState('')
  const [availability, setAvailability] = useState('')
  const [sort, setSort] = useState('name_asc')
  const [page, setPage] = useState(1)
  const [pagination, setPagination] = useState({ total: 0, totalPages: 0 })
  const [productsLoading, setProductsLoading] = useState(false)
  const [marketplaceError, setMarketplaceError] = useState('')
  const [selectedProduct, setSelectedProduct] = useState(null)
  const [productSeller, setProductSeller] = useState(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [cart, setCart] = useState([])
  const [cartSummary, setCartSummary] = useState({ subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
  const [cartError, setCartError] = useState('')
  const [checkoutOpen, setCheckoutOpen] = useState(false)
  const [fulfillmentType, setFulfillmentType] = useState('delivery')
  const [checkoutAddress, setCheckoutAddress] = useState('')
  const [paymentMethod, setPaymentMethod] = useState('simulated')
  const [orderConfirmation, setOrderConfirmation] = useState(null)
  const [authOpen, setAuthOpen] = useState(false)
  const [authMode, setAuthMode] = useState('login')
  const [user, setUser] = useState(readSavedUser)
  const [authForm, setAuthForm] = useState({
    name: '',
    email: '',
    phone: '',
    password: '',
    villageId: '',
    role: 'customer',
  })
  const [authMessage, setAuthMessage] = useState('')
  const [sellerProducts, setSellerProducts] = useState([])
  const [sellerOrders, setSellerOrders] = useState([])
  const [customerOrders, setCustomerOrders] = useState([])
  const [orderTracking, setOrderTracking] = useState({})
  const [orderError, setOrderError] = useState('')
  const [adminStats, setAdminStats] = useState({
    users: 0,
    sellers: 0,
    deliveryAgents: 0,
    products: 0,
    orders: 0,
    revenue: 0,
  })
  const [adminUsers, setAdminUsers] = useState([])
  const [agentDeliveries, setAgentDeliveries] = useState([])
  const [productDraft, setProductDraft] = useState({
    name: '',
    category: '',
    price: '',
    stock: '',
    unit: 'kg',
    villageId: '',
    village: '',
    seller: '',
    available: true,
    description: '',
  })
  const [dashboardMessage, setDashboardMessage] = useState('')

  useEffect(() => {
    const loadData = async () => {
      try {
        const [villagesResponse, categoriesResponse] = await Promise.all([
          axios.get(`${API_BASE_URL}/api/villages`),
          axios.get(`${API_BASE_URL}/api/categories`),
        ])

        setVillages(villagesResponse.data?.villages || [])
        setCategories(categoriesResponse.data?.categories || [])
      } catch (error) {
        setMarketplaceError(error.response?.data?.message || 'Unable to load marketplace locations and categories.')
      }
    }

    loadData()
  }, [])

  const states = useMemo(
    () => [...new Set(villages.map((village) => village.state).filter(Boolean))].sort(),
    [villages],
  )
  const districts = useMemo(
    () => [...new Set(villages.filter((village) => village.state === selectedState).map((village) => village.district).filter(Boolean))].sort(),
    [villages, selectedState],
  )
  const taluks = useMemo(
    () => {
      const districtVillages = villages.filter(
        (village) => village.state === selectedState && village.district === selectedDistrict,
      )
      const names = districtVillages.map((village) => village.taluk || '').filter(Boolean)
      if (districtVillages.some((village) => !village.taluk)) names.push('__unspecified')
      return [...new Set(names)].sort()
    },
    [villages, selectedState, selectedDistrict],
  )
  const villageOptions = useMemo(
    () => villages.filter((village) => village.state === selectedState &&
      village.district === selectedDistrict && (village.taluk || '__unspecified') === selectedTaluk),
    [villages, selectedState, selectedDistrict, selectedTaluk],
  )

  useEffect(() => {
    if (!selectedVillageId) {
      return undefined
    }

    let active = true
    const timeoutId = window.setTimeout(async () => {
      if (!active) return
      setProductsLoading(true)
      setMarketplaceError('')
      try {
        const params = {
          villageId: selectedVillageId,
          page,
          pageSize: PAGE_SIZE,
          sort,
        }
        if (searchText.trim()) params.q = searchText.trim()
        if (selectedCategory) params.category = selectedCategory
        if (minPrice !== '') params.minPrice = minPrice
        if (maxPrice !== '') params.maxPrice = maxPrice
        if (availability !== '') params.available = availability
        const response = await axios.get(`${API_BASE_URL}/api/search/products`, { params })
        if (active) {
          setProducts(response.data.products || [])
          setProductSuggestions(response.data.suggestions || [])
          setPagination(response.data.pagination || { total: 0, totalPages: 0 })
        }
      } catch (error) {
        if (active) {
          setMarketplaceError(error.response?.data?.message || 'Unable to load products for this village.')
          setProducts([])
          setProductSuggestions([])
          setPagination({ total: 0, totalPages: 0 })
        }
      } finally {
        if (active) setProductsLoading(false)
      }
    }, searchText ? 250 : 0)

    return () => {
      active = false
      window.clearTimeout(timeoutId)
    }
  }, [selectedVillageId, selectedCategory, searchText, minPrice, maxPrice, availability, sort, page])

  const selectedProductId = selectedProduct?.id
  useEffect(() => {
    if (!selectedProductId) return undefined
    let active = true
    const loadProductDetails = async () => {
      try {
        const response = await axios.get(`${API_BASE_URL}/api/products/${selectedProductId}`)
        if (active) {
          setSelectedProduct((currentProduct) =>
            currentProduct?.id === selectedProductId ? response.data.product : currentProduct)
        }
        const sellerId = response.data.product?.sellerId
        if (sellerId) {
          const sellerResponse = await axios.get(`${API_BASE_URL}/api/sellers/${sellerId}`)
          if (active) setProductSeller(sellerResponse.data.seller)
        } else if (active) {
          setProductSeller(null)
        }
      } catch (error) {
        if (active) {
          setProductSeller(null)
          setDetailError(error.response?.data?.message || 'Unable to load product details.')
        }
      } finally {
        if (active) setDetailLoading(false)
      }
    }
    loadProductDetails()
    return () => { active = false }
  }, [selectedProductId])

  useEffect(() => {
    const token = localStorage.getItem('villageconnect_token')
    if (!token) return

    const hydrateUser = async () => {
      try {
        const response = await axios.get(`${API_BASE_URL}/api/auth/me`, {
          headers: getAuthHeaders(),
        })
        setUser(response.data.user)
        localStorage.setItem('villageconnect_user', JSON.stringify(response.data.user))
      } catch {
        localStorage.removeItem('villageconnect_token')
        localStorage.removeItem('villageconnect_user')
        setUser(null)
        setCart([])
        setCartSummary({ subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
      }
    }

    hydrateUser()
  }, [])

  useEffect(() => {
    if (user?.role !== 'customer') return undefined
    let active = true
    requestCart(fulfillmentType)
      .then((response) => {
        if (!active) return
        setCart(response.data.cart?.items || [])
        setCartSummary(response.data.cart || { subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
      })
      .catch((error) => {
        if (!active) return
        setCartError(error.response?.data?.message || 'Unable to load your saved cart.')
      })
    return () => { active = false }
  }, [user, fulfillmentType])

  useEffect(() => {
    if (!user) return

    const loadDashboardData = async () => {
      try {
        if (user.role === 'seller') {
          const [productsResponse, ordersResponse] = await Promise.all([
            axios.get(`${API_BASE_URL}/api/sellers/${user.id}/products`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/seller/orders`, { headers: getAuthHeaders() }),
          ])
          setSellerProducts(productsResponse.data.products || [])
          setSellerOrders(ordersResponse.data.orders || [])
        }

        if (user.role === 'customer') {
          const response = await axios.get(`${API_BASE_URL}/api/orders`, { headers: getAuthHeaders() })
          setCustomerOrders(response.data.orders || [])
        }

        if (user.role === 'admin') {
          const [statsResponse, usersResponse] = await Promise.all([
            axios.get(`${API_BASE_URL}/api/admin/analytics`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/users`, { headers: getAuthHeaders() }),
          ])
          setAdminStats(statsResponse.data.stats || {})
          setAdminUsers(usersResponse.data.users || [])
        }

        if (user.role === 'delivery_agent') {
          const response = await axios.get(`${API_BASE_URL}/api/agents/${user.id}/deliveries`, {
            headers: getAuthHeaders(),
          })
          setAgentDeliveries(response.data.deliveries || [])
        }
      } catch (error) {
        console.warn('Unable to load dashboard data:', error)
        setOrderError(error.response?.data?.message || 'Unable to load order information.')
      }
    }

    loadDashboardData()
  }, [user])

  const selectedVillage = villages.find((village) => village.id === selectedVillageId)
  const effectiveProductDraft = useMemo(() => {
    const villageId = productDraft.villageId || user?.villageId || villages[0]?.id || ''
    return {
      ...productDraft,
      seller: user?.name || productDraft.seller,
      villageId,
      village: productDraft.village || villages.find((village) => village.id === villageId)?.name || '',
      category: productDraft.category || categories[0]?.name || '',
    }
  }, [productDraft, user, villages, categories])
  const isAvailable = (product) => product.available && product.stock > 0
  const cartQuantityFor = (productId) => cart.find((item) => item.productId === productId)?.quantity || 0
  const canAddProduct = (product) => isAvailable(product) && cartQuantityFor(product.id) < product.stock
  const showProduct = (product) => {
    setMarketplaceError('')
    setDetailError('')
    setProductSeller(null)
    setDetailLoading(true)
    setSelectedProduct(product)
  }
  const closeProduct = () => {
    setSelectedProduct(null)
    setProductSeller(null)
    setDetailError('')
    setDetailLoading(false)
  }
  const clearMarketplaceProducts = () => {
    setProducts([])
    setProductSuggestions([])
    setPagination({ total: 0, totalPages: 0 })
    setProductsLoading(false)
  }

  const cartCount = cart.reduce((sum, item) => sum + item.quantity, 0)
  const subtotal = cartSummary.subtotal || 0
  const deliveryFee = cartSummary.deliveryFee || 0
  const total = cartSummary.total || 0

  const syncCart = async () => {
    const response = await requestCart(fulfillmentType)
    setCart(response.data.cart?.items || [])
    setCartSummary(response.data.cart || { subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
  }

  const addToCart = async (product) => {
    if (!canAddProduct(product)) return
    if (user?.role !== 'customer') {
      if (!user) {
        setAuthMode('login')
        setAuthOpen(true)
        setAuthMessage('Please sign in as a customer to save items to your cart.')
      } else setCartError('Only customer accounts can use the cart.')
      return
    }
    setCartError('')
    setOrderConfirmation(null)
    try {
      await axios.post(`${API_BASE_URL}/api/cart/items`, {
        productId: product.id,
        villageId: selectedVillageId,
        quantity: 1,
      }, { headers: getAuthHeaders() })
      await syncCart()
    } catch (error) {
      setCartError(error.response?.data?.message || 'Unable to add this product to your cart.')
    }
  }

  const updateQuantity = async (productId, delta) => {
    const current = cart.find((item) => item.productId === productId)
    if (!current) return
    setCartError('')
    try {
      if (current.quantity + delta <= 0) {
        await axios.delete(`${API_BASE_URL}/api/cart/items/${productId}`, { headers: getAuthHeaders() })
      } else {
        await axios.put(`${API_BASE_URL}/api/cart/items/${productId}`,
          { quantity: current.quantity + delta }, { headers: getAuthHeaders() })
      }
      await syncCart()
    } catch (error) {
      setCartError(error.response?.data?.message || 'Unable to update your cart.')
    }
  }

  const clearCart = async () => {
    setCartError('')
    try {
      await axios.delete(`${API_BASE_URL}/api/cart`, { headers: getAuthHeaders() })
      await syncCart()
    } catch (error) {
      setCartError(error.response?.data?.message || 'Unable to clear your cart.')
    }
  }

  const placeOrder = () => {
    if (!cart.length || cart.some((item) => !item.valid)) return
    if (user?.role !== 'customer') {
      setAuthMode('login')
      setAuthOpen(true)
      setAuthMessage('Please sign in as a customer before checking out.')
      return
    }
    setCheckoutOpen(true)
    setCartError('')
  }

  const submitCheckout = async (event) => {
    event.preventDefault()
    if (!cart.length) return
    if (fulfillmentType === 'delivery' && !checkoutAddress.trim()) {
      setCartError('Enter a delivery address to continue.')
      return
    }
    setCartError('')
    try {
      const response = await axios.post(`${API_BASE_URL}/api/cart/checkout`, {
        villageId: cartSummary.villageId || selectedVillageId,
        fulfillmentType,
        address: fulfillmentType === 'delivery' ? checkoutAddress : '',
        paymentMethod,
      }, { headers: getAuthHeaders() })
      setOrderConfirmation(response.data.order)
      setCart([])
      setCartSummary({ subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
      setCheckoutOpen(false)
      setCheckoutAddress('')
    } catch (error) {
      setCartError(error.response?.data?.message || 'Checkout could not be completed. Review your cart and try again.')
      try {
        await syncCart()
      } catch (syncError) {
        console.warn('Unable to refresh cart after checkout failure:', syncError)
      }
    }
  }

  const handleAuthChange = (event) => {
    const { name, value } = event.target
    setAuthForm((currentForm) => ({ ...currentForm, [name]: value }))
  }

  const handleAuthSubmit = async (event) => {
    event.preventDefault()

    const payload = authMode === 'login'
      ? { email: authForm.email, password: authForm.password }
      : {
          name: authForm.name,
          email: authForm.email,
          phone: authForm.phone,
          password: authForm.password,
          villageId: authForm.villageId,
          role: authForm.role,
        }

    try {
      const endpoint = authMode === 'login' ? '/api/auth/login' : '/api/auth/register'
      const response = await axios.post(`${API_BASE_URL}${endpoint}`, payload)
      const token = response.data.token
      const userPayload = response.data.user

      localStorage.setItem('villageconnect_token', token)
      localStorage.setItem('villageconnect_user', JSON.stringify(userPayload))
      setUser(userPayload)
      setAuthMessage(authMode === 'login' ? 'Signed in successfully.' : 'Account created successfully.')
      setAuthOpen(false)
      setAuthForm({
        name: '',
        email: '',
        phone: '',
        password: '',
        villageId: '',
        role: 'customer',
      })
    } catch (error) {
      const message = error.response?.data?.message || 'Authentication failed.'
      setAuthMessage(message)
    }
  }

  const handleProductDraftChange = (event) => {
    const { name, value, type, checked } = event.target
    if (name === 'villageId') {
      const village = villages.find((option) => option.id === value)
      setProductDraft((currentDraft) => ({
        ...currentDraft,
        villageId: value,
        village: village?.name || '',
      }))
      return
    }
    setProductDraft((currentDraft) => ({
      ...currentDraft,
      [name]: type === 'checkbox' ? checked : value,
    }))
  }

  const handleSellerProductCreate = async (event) => {
    event.preventDefault()

    if (!user || user.role !== 'seller') return

    try {
      const payload = {
        name: effectiveProductDraft.name,
        category: effectiveProductDraft.category,
        price: Number(effectiveProductDraft.price),
        unit: effectiveProductDraft.unit,
        villageId: effectiveProductDraft.villageId,
        village: effectiveProductDraft.village,
        seller: user.name,
        available: effectiveProductDraft.available,
        stock: Number(effectiveProductDraft.stock),
        description: effectiveProductDraft.description,
      }

      await axios.post(`${API_BASE_URL}/api/products`, payload, { headers: getAuthHeaders() })
      setDashboardMessage('Product listed successfully.')
      setProductDraft({
        name: '',
        category: categories[0]?.name || '',
        price: '',
        stock: '',
        unit: 'kg',
        villageId: user.villageId || villages[0]?.id || '',
        village: villages.find((village) => village.id === (user.villageId || villages[0]?.id))?.name || '',
        seller: user.name,
        available: true,
        description: '',
      })

      const response = await axios.get(`${API_BASE_URL}/api/sellers/${user.id}/products`, {
        headers: getAuthHeaders(),
      })
      setSellerProducts(response.data.products || [])
    } catch (error) {
      console.warn('Unable to create product:', error)
      setDashboardMessage(error.response?.data?.message || 'Unable to list this product.')
    }
  }

  const handleDeliveryStatusUpdate = async (deliveryId, nextStatus) => {
    try {
      await axios.patch(
        `${API_BASE_URL}/api/deliveries/${deliveryId}/status`,
        { status: nextStatus },
        { headers: getAuthHeaders() },
      )
      const response = await axios.get(`${API_BASE_URL}/api/agents/${user.id}/deliveries`, {
        headers: getAuthHeaders(),
      })
      setAgentDeliveries(response.data.deliveries || [])
    } catch (error) {
      console.warn('Unable to update delivery status:', error)
    }
  }

  const handleSellerOrderStatus = async (orderId, status) => {
    setOrderError('')
    try {
      await axios.patch(`${API_BASE_URL}/api/seller/orders/${orderId}/status`, { status }, {
        headers: getAuthHeaders(),
      })
      const response = await axios.get(`${API_BASE_URL}/api/seller/orders`, { headers: getAuthHeaders() })
      setSellerOrders(response.data.orders || [])
    } catch (error) {
      setOrderError(error.response?.data?.message || 'Unable to update this order.')
    }
  }

  const cancelCustomerOrder = async (orderId) => {
    setOrderError('')
    try {
      await axios.patch(`${API_BASE_URL}/api/orders/${orderId}/status`, { status: 'cancelled' }, {
        headers: getAuthHeaders(),
      })
      const response = await axios.get(`${API_BASE_URL}/api/orders`, { headers: getAuthHeaders() })
      setCustomerOrders(response.data.orders || [])
    } catch (error) {
      setOrderError(error.response?.data?.message || 'Unable to cancel this order.')
    }
  }

  const loadOrderTracking = async (orderId) => {
    setOrderError('')
    try {
      const response = await axios.get(`${API_BASE_URL}/api/orders/${orderId}/tracking`, {
        headers: getAuthHeaders(),
      })
      setOrderTracking((current) => ({ ...current, [orderId]: response.data.tracking }))
    } catch (error) {
      setOrderError(error.response?.data?.message || 'Unable to load order tracking.')
    }
  }

  const handleLogout = () => {
    localStorage.removeItem('villageconnect_token')
    localStorage.removeItem('villageconnect_user')
    setUser(null)
    setCart([])
    setCartSummary({ subtotal: 0, deliveryFee: 0, total: 0, villageId: '' })
    setOrderConfirmation(null)
    setCheckoutOpen(false)
    setSellerOrders([])
    setCustomerOrders([])
    setOrderTracking({})
    setOrderError('')
    setSellerProducts([])
    setAdminStats({ users: 0, sellers: 0, deliveryAgents: 0, products: 0, orders: 0, revenue: 0 })
    setAdminUsers([])
    setAgentDeliveries([])
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
          {user ? (
            <>
              <div className="user-badge">
                <span>{user.name}</span>
                <small>{user.role}</small>
              </div>
              <button type="button" className="secondary-button" onClick={handleLogout}>
                Sign out
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                className="secondary-button"
                onClick={() => {
                  setAuthMode('login')
                  setAuthOpen(true)
                }}
              >
                Sign in
              </button>
              <button
                type="button"
                className="primary-button"
                onClick={() => {
                  setAuthMode('register')
                  setAuthOpen(true)
                }}
              >
                Join now
              </button>
            </>
          )}
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
                list="marketplace-product-suggestions"
                value={searchText}
                onChange={(event) => {
                  setSearchText(event.target.value)
                  setPage(1)
                }}
                placeholder="Search tomatoes, rice, fruits..."
              />
              <datalist id="marketplace-product-suggestions">
                {productSuggestions.map((suggestion) => <option key={suggestion} value={suggestion} />)}
              </datalist>
            </div>

            <div className="hero-meta">
              <div>
                <strong>{villages.length} villages</strong>
                <span>Available locations</span>
              </div>
              <div>
                <strong>{categories.length} categories</strong>
                <span>Browse local goods</span>
              </div>
              <div>
                <strong>Village-first</strong>
                <span>Local marketplace</span>
              </div>
            </div>
          </div>

          <div className="hero-panel">
            <div className="panel-header">
              <MapPin size={18} />
              <span>Delivery to</span>
            </div>

            <div className="location-selector">
              <label>
                State
                <select value={selectedState} onChange={(event) => {
                  clearMarketplaceProducts()
                  closeProduct()
                  setSelectedState(event.target.value)
                  setSelectedDistrict('')
                  setSelectedTaluk('')
                  setSelectedVillageId('')
                  setPage(1)
                }}>
                  <option value="">Choose a state</option>
                  {states.map((state) => <option key={state} value={state}>{state}</option>)}
                </select>
              </label>
              <label>
                District
                <select value={selectedDistrict} disabled={!selectedState} onChange={(event) => {
                  clearMarketplaceProducts()
                  closeProduct()
                  setSelectedDistrict(event.target.value)
                  setSelectedTaluk('')
                  setSelectedVillageId('')
                  setPage(1)
                }}>
                  <option value="">Choose a district</option>
                  {districts.map((district) => <option key={district} value={district}>{district}</option>)}
                </select>
              </label>
              <label>
                Taluk
                <select value={selectedTaluk} disabled={!selectedDistrict} onChange={(event) => {
                  clearMarketplaceProducts()
                  closeProduct()
                  setSelectedTaluk(event.target.value)
                  setSelectedVillageId('')
                  setPage(1)
                }}>
                  <option value="">Choose a taluk</option>
                  {taluks.map((taluk) => <option key={taluk} value={taluk}>
                    {taluk === '__unspecified' ? 'Taluk not recorded' : taluk}
                  </option>)}
                </select>
              </label>
              <label>
                Village
                <select value={selectedVillageId} disabled={!selectedTaluk} onChange={(event) => {
                  clearMarketplaceProducts()
                  closeProduct()
                  setSelectedVillageId(event.target.value)
                  setPage(1)
                }}>
                  <option value="">Choose a village</option>
                  {villageOptions.map((village) => (
                    <option key={village.id} value={village.id}>{village.name}</option>
                  ))}
                </select>
              </label>
            </div>
          </div>
        </section>

        <section className="category-strip" aria-label="Categories">
          <button
            type="button"
            className={!selectedCategory ? 'category-pill active' : 'category-pill'}
            onClick={() => {
              setSelectedCategory('')
              setPage(1)
            }}
          >
            All
          </button>
          {categories.map((category) => (
            <button
              key={category.id}
              type="button"
              className={selectedCategory === category.name ? 'category-pill active' : 'category-pill'}
              onClick={() => {
                setSelectedCategory(category.name)
                setPage(1)
              }}
            >
              {category.name}
            </button>
          ))}
        </section>

        <section className="marketplace-section" id="marketplace">
          <div className="product-column">
            {selectedProduct ? (
              <div className="product-detail-page">
                <button type="button" className="secondary-button" onClick={closeProduct}>
                  Back to products
                </button>
                {detailError && <p className="marketplace-message error" role="alert">{detailError}</p>}
                {detailLoading ? <p className="marketplace-message">Loading product details...</p> : (
                  <div className="product-detail-layout">
                    <div className="product-detail-visual" aria-hidden="true">
                      {selectedProduct.category === 'Fruits' ? '🍎' : selectedProduct.category === 'Grains' ? '🌾' : selectedProduct.category === 'Dairy' ? '🥛' : '🥬'}
                    </div>
                    <div className="product-detail-copy">
                      <span className="tag">{selectedProduct.category}</span>
                      <h2>{selectedProduct.name}</h2>
                      <p>{selectedProduct.description}</p>
                      <p className="detail-rating"><Star size={15} fill="currentColor" /> {selectedProduct.rating || 'Not rated'}</p>
                      <div className="detail-price">{formatCurrency(selectedProduct.price)} <small>per {selectedProduct.unit}</small></div>
                      <div className={isAvailable(selectedProduct) ? 'stock-indicator available' : 'stock-indicator unavailable'}>
                        {isAvailable(selectedProduct)
                          ? `Available · ${selectedProduct.stock} in stock`
                          : 'Currently unavailable'}
                      </div>
                      {selectedVillage && <p className="detail-location">Available in {selectedVillage.name} village</p>}
                      <button
                        type="button"
                        className="primary-button"
                        disabled={!canAddProduct(selectedProduct)}
                        onClick={() => addToCart(selectedProduct)}
                      >
                        {!isAvailable(selectedProduct) ? 'Out of stock' :
                          canAddProduct(selectedProduct) ? 'Add to cart' : 'Stock limit reached'}
                      </button>
                      <section className="seller-detail-card">
                        <span className="section-kicker">Seller information</span>
                        <h3>{productSeller?.name || selectedProduct.seller}</h3>
                        {productSeller ? (
                          <>
                            <p>Verified VillageConnect seller</p>
                            <span>
                              {villages.find((village) => village.id === productSeller.villageId)?.name ||
                                'Seller village not provided'}
                            </span>
                          </>
                        ) : (
                          <p>Seller account details are not linked to this legacy product listing.</p>
                        )}
                      </section>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <>
                <div className="section-header">
                  <div>
                    <span className="section-kicker">Marketplace</span>
                    <h2>{selectedVillage ? `Fresh goods in ${selectedVillage.name}` : 'Choose a village to browse'}</h2>
                  </div>
                  <span className="result-count">{pagination.total} products</span>
                </div>

                <div className="marketplace-filters">
                  <label>
                    Minimum price
                    <input type="number" min="0" value={minPrice} onChange={(event) => {
                      setMinPrice(event.target.value)
                      setPage(1)
                    }} placeholder="Any" />
                  </label>
                  <label>
                    Maximum price
                    <input type="number" min="0" value={maxPrice} onChange={(event) => {
                      setMaxPrice(event.target.value)
                      setPage(1)
                    }} placeholder="Any" />
                  </label>
                  <label>
                    Availability
                    <select value={availability} onChange={(event) => {
                      setAvailability(event.target.value)
                      setPage(1)
                    }}>
                      <option value="">All items</option>
                      <option value="true">Available only</option>
                      <option value="false">Unavailable</option>
                    </select>
                  </label>
                  <label>
                    Sort by
                    <select value={sort} onChange={(event) => {
                      setSort(event.target.value)
                      setPage(1)
                    }}>
                      <option value="name_asc">Name: A to Z</option>
                      <option value="name_desc">Name: Z to A</option>
                      <option value="price_asc">Price: low to high</option>
                      <option value="price_desc">Price: high to low</option>
                      <option value="rating_desc">Top rated</option>
                    </select>
                  </label>
                </div>

                {marketplaceError && <p className="marketplace-message error">{marketplaceError}</p>}
                {productsLoading ? <p className="marketplace-message">Loading village products...</p> : selectedVillageId ? (
                  products.length ? (
                    <>
                      <div className="product-grid">
                        {products.map((product) => (
                          <article key={product.id} className="product-card">
                            <button type="button" className="product-visual product-detail-trigger" onClick={() => showProduct(product)} aria-label={`View details for ${product.name}`}>
                              {product.category === 'Fruits' ? '🍎' : product.category === 'Grains' ? '🌾' : product.category === 'Dairy' ? '🥛' : '🥬'}
                            </button>

                            <div className="product-body">
                              <div className="product-topline">
                                <span className="tag">{product.category}</span>
                                <span className="rating">
                                  <Star size={12} fill="currentColor" /> {product.rating || 'New'}
                                </span>
                              </div>

                              <button type="button" className="product-name-button" onClick={() => showProduct(product)}>
                                <h3>{product.name}</h3>
                              </button>
                              <p>{product.description}</p>

                              <div className="product-meta">
                                <span>{product.seller}</span>
                                <span>{product.village}</span>
                              </div>
                              <div className={isAvailable(product) ? 'stock-indicator available' : 'stock-indicator unavailable'}>
                                {isAvailable(product) ? `${product.stock} in stock` : 'Unavailable'}
                              </div>

                              <div className="product-footer">
                                <div>
                                  <strong>{formatCurrency(product.price)}</strong>
                                  <small>per {product.unit}</small>
                                </div>
                                <button
                                  type="button"
                                  className="primary-button small"
                                  disabled={!canAddProduct(product)}
                                  onClick={() => addToCart(product)}
                                >
                                  {!isAvailable(product) ? 'Out of stock' :
                                    canAddProduct(product) ? 'Add to cart' : 'Stock limit reached'}
                                </button>
                              </div>
                            </div>
                          </article>
                        ))}
                      </div>
                      <div className="pagination-controls">
                        <button type="button" className="secondary-button" disabled={page <= 1} onClick={() => setPage((currentPage) => currentPage - 1)}>
                          Previous
                        </button>
                        <span>Page {page} of {Math.max(pagination.totalPages, 1)}</span>
                        <button type="button" className="secondary-button" disabled={page >= pagination.totalPages} onClick={() => setPage((currentPage) => currentPage + 1)}>
                          Next
                        </button>
                      </div>
                    </>
                  ) : <p className="marketplace-message">No products match your selection in this village.</p>
                ) : <p className="marketplace-message">Choose State, District, Taluk, and Village to see local products.</p>}
              </>
            )}
          </div>

          <aside className="cart-panel">
            <div className="cart-header">
              <div className="cart-title-wrap">
                <ShoppingCart size={18} />
                <h3>Cart</h3>
              </div>
              <span className="cart-badge">{cartCount}</span>
            </div>

            {cartError && <p className="cart-error" role="alert">{cartError}</p>}
            {orderConfirmation ? (
              <div className="order-confirmation" role="status">
                <CheckCircle2 size={24} />
                <h4>Order confirmed</h4>
                <p>Order {orderConfirmation.id} has been placed.</p>
                <strong>{formatCurrency(orderConfirmation.total)}</strong>
                <span>Payment: {orderConfirmation.payment?.status || 'pending'}</span>
                <button type="button" className="secondary-button" onClick={() => setOrderConfirmation(null)}>
                  Continue shopping
                </button>
              </div>
            ) : cart.length === 0 ? (
              <div className="empty-cart">
                <p>Your cart is empty.</p>
                <span>Add fresh items from nearby sellers.</span>
              </div>
            ) : (
              <div className="cart-items">
                {cart.map((item) => (
                  <div key={item.productId} className={`cart-item${item.valid ? '' : ' invalid-cart-item'}`}>
                    <div>
                      <strong>{item.name || 'Unavailable product'}</strong>
                      {item.name && <span>{formatCurrency(item.price)} / {item.unit}</span>}
                      {item.issue && <span className="cart-item-issue">{item.issue}</span>}
                    </div>

                    <div className="quantity-controls">
                      <button type="button" onClick={() => updateQuantity(item.productId, -1)} aria-label={`Decrease ${item.name || 'item'} quantity`}>-</button>
                      <span>{item.quantity}</span>
                      <button
                        type="button"
                        disabled={item.quantity >= item.stock}
                        onClick={() => updateQuantity(item.productId, 1)}
                        aria-label={`Increase ${item.name || 'item'} quantity`}
                      >
                        +
                      </button>
                      <button type="button" className="remove-cart-item" onClick={() => updateQuantity(item.productId, -item.quantity)}>
                        Remove
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {!orderConfirmation && (
              <>
                <div className="order-summary">
                  <div>
                    <span>Subtotal</span>
                    <strong>{formatCurrency(subtotal)}</strong>
                  </div>
                  <div>
                    <span>{fulfillmentType === 'delivery' ? 'Delivery fee' : 'Pickup fee'}</span>
                    <strong>{formatCurrency(deliveryFee)}</strong>
                  </div>
                  <div className="summary-total">
                    <span>Total</span>
                    <strong>{formatCurrency(total)}</strong>
                  </div>
                </div>
                {cart.length > 0 && (
                  <div className="checkout-options">
                    <label htmlFor="fulfillment-type">Fulfillment</label>
                    <select id="fulfillment-type" value={fulfillmentType}
                      onChange={(event) => {
                        setFulfillmentType(event.target.value)
                        setPaymentMethod('simulated')
                      }}>
                      <option value="delivery">Delivery to my address</option>
                      <option value="pickup">Pick up from seller</option>
                    </select>
                    {fulfillmentType === 'delivery' && (
                      <label>
                        Delivery address
                        <textarea value={checkoutAddress} onChange={(event) => setCheckoutAddress(event.target.value)}
                          maxLength={500} required placeholder="Enter your complete village address" />
                      </label>
                    )}
                    <label htmlFor="payment-method">Payment</label>
                    <select id="payment-method" value={paymentMethod} onChange={(event) => setPaymentMethod(event.target.value)}>
                      <option value="simulated">Simulated online payment</option>
                      {fulfillmentType === 'delivery'
                        ? <option value="cash_on_delivery">Cash on delivery</option>
                        : <option value="cash_on_pickup">Cash on pickup</option>}
                    </select>
                  </div>
                )}
                <button type="button" className="checkout-button" onClick={placeOrder}
                  disabled={!cart.length || cart.some((item) => !item.valid)}>
                  Checkout
                </button>
                {cart.length > 0 && (
                  <button type="button" className="clear-cart-button" onClick={clearCart}>Clear cart</button>
                )}
                {checkoutOpen && cart.length > 0 && (
                  <form className="checkout-confirm-form" onSubmit={submitCheckout}>
                    <p>Confirm checkout for {villages.find((village) => village.id === cartSummary.villageId)?.name || 'your selected village'}?</p>
                    <button type="submit" className="checkout-button">Place order · {formatCurrency(total)}</button>
                    <button type="button" className="secondary-button" onClick={() => setCheckoutOpen(false)}>Go back</button>
                  </form>
                )}
              </>
            )}
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

      {user && (
        <section className="dashboard-section" aria-label={`${user.role} dashboard`}>
          {user.role === 'seller' && (
            <div className="dashboard-grid">
              <div className="dashboard-card wide-card">
                <h3>Seller performance</h3>
                <div className="dashboard-grid">
                  <div className="stat-card">
                    <span>Listings</span>
                    <strong>{sellerProducts.length}</strong>
                  </div>
                  <div className="stat-card">
                    <span>Inventory</span>
                    <strong>{sellerProducts.reduce((sum, product) => sum + product.stock, 0)}</strong>
                  </div>
                </div>
              </div>

              <div className="dashboard-card">
                <h3>Create a product</h3>
                <form className="dashboard-form" onSubmit={handleSellerProductCreate}>
                  <input name="name" value={effectiveProductDraft.name} onChange={handleProductDraftChange} placeholder="Product name" required />
                  <select name="category" value={effectiveProductDraft.category} onChange={handleProductDraftChange} required>
                    {!effectiveProductDraft.category && <option value="">Choose a category</option>}
                    {categories.map((category) => (
                      <option key={category.id} value={category.name}>{category.name}</option>
                    ))}
                  </select>
                  <input name="price" type="number" min="1" value={effectiveProductDraft.price} onChange={handleProductDraftChange} placeholder="Price" required />
                  <input name="stock" type="number" min="0" value={effectiveProductDraft.stock} onChange={handleProductDraftChange} placeholder="Stock" required />
                  <input name="unit" value={effectiveProductDraft.unit} onChange={handleProductDraftChange} placeholder="Unit (kg/dozen)" required />
                  <select name="villageId" value={effectiveProductDraft.villageId} onChange={handleProductDraftChange} required>
                    <option value="">Choose a village</option>
                    {villages.map((village) => (
                      <option key={village.id} value={village.id}>{village.name}</option>
                    ))}
                  </select>
                  <textarea name="description" value={effectiveProductDraft.description} onChange={handleProductDraftChange} placeholder="Product description" required />
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 700, color: '#234d3b' }}>
                    <input name="available" type="checkbox" checked={effectiveProductDraft.available} onChange={handleProductDraftChange} />
                    Available now
                  </label>
                  <button type="submit" className="primary-button full-width">List product</button>
                </form>
                {dashboardMessage && <p className="auth-message" style={{ marginTop: '10px' }}>{dashboardMessage}</p>}
              </div>

              <div className="dashboard-card wide-card">
                <h3>Your listings</h3>
                <div className="mini-list">
                  {sellerProducts.length === 0 ? (
                    <p className="auth-message">No products listed yet.</p>
                  ) : (
                    sellerProducts.map((product) => (
                      <div key={product.id} className="mini-list-item">
                        <div>
                          <strong>{product.name}</strong>
                          <span>{product.village} · {product.category}</span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span>{product.stock} in stock</span>
                          <span className="status-pill">{product.available ? 'available' : 'out of stock'}</span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>

              <div className="dashboard-card wide-card">
                <h3>Incoming orders</h3>
                {orderError && <p className="cart-error" role="alert">{orderError}</p>}
                <div className="mini-list">
                  {sellerOrders.length === 0 ? (
                    <p className="auth-message">No customer orders yet.</p>
                  ) : sellerOrders.map((order) => {
                    const actions = {
                      pending: [['confirmed', 'Accept'], ['rejected', 'Reject']],
                      confirmed: [['preparing', 'Start preparing']],
                      preparing: [['packed', 'Mark packed']],
                      packed: [['ready_for_pickup', 'Mark ready']],
                    }[order.status] || []
                    return (
                      <div key={order.id} className="mini-list-item order-list-item">
                        <div>
                          <strong>Order {order.id}</strong>
                          <span>{order.items?.map((item) => `${item.name} × ${item.quantity}`).join(', ')}</span>
                          <span>{order.fulfillmentType} · {order.villageId}</span>
                        </div>
                        <div className="order-actions">
                          <span className="status-pill">{order.status}</span>
                          <strong>{formatCurrency(order.total)}</strong>
                          {actions.map(([status, label]) => (
                            <button key={status} type="button" className={status === 'rejected' ? 'secondary-button small' : 'primary-button small'}
                              onClick={() => handleSellerOrderStatus(order.id, status)}>
                              {label}
                            </button>
                          ))}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          )}

          {user.role === 'customer' && (
            <div className="dashboard-grid">
              <div className="dashboard-card wide-card">
                <h3>My orders</h3>
                {orderError && <p className="cart-error" role="alert">{orderError}</p>}
                <div className="mini-list">
                  {customerOrders.length === 0 ? (
                    <p className="auth-message">Your placed orders will appear here.</p>
                  ) : customerOrders.map((order) => {
                    const tracking = orderTracking[order.id]
                    return (
                      <div key={order.id} className="mini-list-item order-list-item">
                        <div>
                          <strong>Order {order.id}</strong>
                          <span>{order.items?.map((item) => `${item.name} × ${item.quantity}`).join(', ')}</span>
                          <span>{order.fulfillmentType} · {order.status}</span>
                          {tracking?.statusHistory?.map((event, index) => (
                            <span key={`${event.status}-${event.createdAt}-${index}`} className="tracking-event">
                              {event.status} · {new Date(event.createdAt).toLocaleString('en-IN')}
                            </span>
                          ))}
                        </div>
                        <div className="order-actions">
                          <strong>{formatCurrency(order.total)}</strong>
                          <button type="button" className="secondary-button small" onClick={() => loadOrderTracking(order.id)}>
                            Track order
                          </button>
                          {['pending', 'confirmed'].includes(order.status) && (
                            <button type="button" className="secondary-button small" onClick={() => cancelCustomerOrder(order.id)}>
                              Cancel
                            </button>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          )}

          {user.role === 'admin' && (
            <div className="dashboard-grid">
              <div className="stat-card"><span>Total users</span><strong>{adminStats.users || 0}</strong></div>
              <div className="stat-card"><span>Sellers</span><strong>{adminStats.sellers || 0}</strong></div>
              <div className="stat-card"><span>Delivery agents</span><strong>{adminStats.deliveryAgents || 0}</strong></div>
              <div className="stat-card"><span>Products</span><strong>{adminStats.products || 0}</strong></div>
              <div className="stat-card"><span>Orders</span><strong>{adminStats.orders || 0}</strong></div>
              <div className="stat-card"><span>Revenue</span><strong>{formatCurrency(Number(adminStats.revenue || 0))}</strong></div>

              <div className="dashboard-card wide-card">
                <h3>Community overview</h3>
                <div className="mini-list">
                  {adminUsers.length === 0 ? (
                    <p className="auth-message">No users available.</p>
                  ) : (
                    adminUsers.map((member) => (
                      <div key={member.id} className="mini-list-item">
                        <div>
                          <strong>{member.name}</strong>
                          <span>{member.email}</span>
                        </div>
                        <span className="status-pill">{member.role}</span>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}

          {user.role === 'delivery_agent' && (
            <div className="dashboard-grid">
              <div className="dashboard-card wide-card">
                <h3>Assigned deliveries</h3>
                <div className="mini-list">
                  {agentDeliveries.length === 0 ? (
                    <p className="auth-message">No deliveries assigned yet.</p>
                  ) : (
                    agentDeliveries.map((delivery) => (
                      <div key={delivery.id} className="mini-list-item">
                        <div>
                          <strong>{delivery.id}</strong>
                          <span>Order: {delivery.orderId}</span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span className="status-pill">{delivery.status}</span>
                          <button
                            type="button"
                            className="secondary-button small"
                            onClick={() => handleDeliveryStatusUpdate(delivery.id, delivery.status === 'assigned' ? 'in_transit' : 'delivered')}
                          >
                            {delivery.status === 'assigned' ? 'Mark in transit' : 'Mark delivered'}
                          </button>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}
        </section>
      )}

      {authOpen && (
        <div className="auth-modal-backdrop" onClick={() => setAuthOpen(false)}>
          <div className="auth-modal" onClick={(event) => event.stopPropagation()}>
            <div className="auth-header">
              <div>
                <span className="section-kicker">VillageConnect</span>
                <h3>{authMode === 'login' ? 'Welcome back' : 'Create your account'}</h3>
              </div>
              <button type="button" className="close-button" onClick={() => setAuthOpen(false)}>?</button>
            </div>

            <form onSubmit={handleAuthSubmit} className="auth-form">
              {authMode === 'register' && (
                <label>
                  Full name
                  <input name="name" value={authForm.name} onChange={handleAuthChange} placeholder="Your name" required />
                </label>
              )}

              <label>
                Email
                <input name="email" type="email" value={authForm.email} onChange={handleAuthChange} placeholder="name@example.com" required />
              </label>

              {authMode === 'register' && (
                <>
                  <label>
                    Phone
                    <input name="phone" value={authForm.phone} onChange={handleAuthChange} placeholder="Phone number" />
                  </label>

                  <label>
                    Village
                    <select name="villageId" value={authForm.villageId} onChange={handleAuthChange}>
                      {villages.map((village) => (
                        <option key={village.id} value={village.id}>{village.name}</option>
                      ))}
                    </select>
                  </label>

                </>
              )}

              <label>
                Password
                <input name="password" type="password" value={authForm.password} onChange={handleAuthChange} placeholder="Password" required />
              </label>

              {authMessage && <p className="auth-message">{authMessage}</p>}

              <button type="submit" className="primary-button full-width">
                {authMode === 'login' ? 'Sign in' : 'Create account'}
              </button>

              <button
                type="button"
                className="secondary-button full-width"
                onClick={() => setAuthMode(authMode === 'login' ? 'register' : 'login')}
              >
                {authMode === 'login' ? 'Need an account? Join now' : 'Already have an account? Sign in'}
              </button>
            </form>
          </div>
        </div>
      )}

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

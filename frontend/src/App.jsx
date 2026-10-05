import { useEffect, useMemo, useState } from 'react'
import axios from 'axios'
import {
  CheckCircle2,
  ChevronRight,
  Bell,
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

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8081'
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

const customerTimeline = (order, tracking) => {
  const orderEvents = order.statusHistory || []
  const delivery = tracking?.delivery
  const deliveryEvents = delivery?.history || []
  const orderHas = (...statuses) => orderEvents.some((event) => statuses.includes(event.status))
  const deliveryHas = (status) => delivery?.status === status || deliveryEvents.some((event) => event.status === status)
  const orderTime = (...statuses) => orderEvents.find((event) => statuses.includes(event.status))?.createdAt || ''
  const deliveryTime = (status, fallback = '') =>
    deliveryEvents.find((event) => event.status === status)?.createdAt || fallback

  return [
    { label: 'Order Placed', complete: true, timestamp: orderTime('pending', 'placed') || order.createdAt },
    { label: 'Confirmed', complete: orderHas('confirmed'), timestamp: orderTime('confirmed') },
    { label: 'Preparing', complete: orderHas('preparing'), timestamp: orderTime('preparing') },
    { label: 'Packed', complete: orderHas('packed', 'ready_for_pickup'), timestamp: orderTime('packed', 'ready_for_pickup') },
    { label: 'Assigned', complete: Boolean(delivery), timestamp: delivery?.createdAt || '' },
    { label: 'Picked Up', complete: deliveryHas('picked_up'), timestamp: deliveryTime('picked_up', delivery?.pickedUpAt) },
    { label: 'In Transit', complete: deliveryHas('in_transit'), timestamp: deliveryTime('in_transit', delivery?.inTransitAt) },
    { label: 'Delivered', complete: order.status === 'delivered', timestamp: orderTime('delivered') || delivery?.customerConfirmedAt || '' },
  ]
}

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
  const [sellerSection, setSellerSection] = useState('Overview')
  const [productEditId, setProductEditId] = useState('')
  const [selectedSellerOrderId, setSelectedSellerOrderId] = useState('')
  const [customerOrders, setCustomerOrders] = useState([])
  const [orderTracking, setOrderTracking] = useState({})
  const [expandedCustomerOrderId, setExpandedCustomerOrderId] = useState('')
  const [orderSellerProfiles, setOrderSellerProfiles] = useState({})
  const [orderReviews, setOrderReviews] = useState({})
  const [reviewMessage, setReviewMessage] = useState('')
  const [notifications, setNotifications] = useState([])
  const [unreadNotificationCount, setUnreadNotificationCount] = useState(0)
  const [notificationsOpen, setNotificationsOpen] = useState(false)
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
  const [adminProducts, setAdminProducts] = useState([])
  const [adminOrders, setAdminOrders] = useState([])
  const [adminVillages, setAdminVillages] = useState([])
  const [adminCategories, setAdminCategories] = useState([])
  const [adminDeliveries, setAdminDeliveries] = useState([])
  const [adminSection, setAdminSection] = useState('Overview')
  const [adminUserSearch, setAdminUserSearch] = useState('')
  const [adminUserRole, setAdminUserRole] = useState('all')
  const [adminOrderSearch, setAdminOrderSearch] = useState('')
  const [adminOrderStatus, setAdminOrderStatus] = useState('all')
  const [adminExpandedOrderId, setAdminExpandedOrderId] = useState('')
  const [adminVillageEditId, setAdminVillageEditId] = useState('')
  const [adminVillageDraft, setAdminVillageDraft] = useState({ name: '', district: '', taluk: '', state: '' })
  const [adminCategoryEditId, setAdminCategoryEditId] = useState('')
  const [adminCategoryDraft, setAdminCategoryDraft] = useState({ name: '' })
  const [adminMessage, setAdminMessage] = useState('')
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
          const [statsResponse, usersResponse, productsResponse, ordersResponse, villagesResponse, categoriesResponse, deliveriesResponse] = await Promise.all([
            axios.get(`${API_BASE_URL}/api/admin/analytics`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/users`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/products`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/orders`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/villages`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/admin/categories`, { headers: getAuthHeaders() }),
            axios.get(`${API_BASE_URL}/api/deliveries`, { headers: getAuthHeaders() }),
          ])
          setAdminStats(statsResponse.data.stats || {})
          setAdminUsers(usersResponse.data.users || [])
          setAdminProducts(productsResponse.data.products || [])
          setAdminOrders(ordersResponse.data.orders || [])
          setAdminVillages(villagesResponse.data.villages || [])
          setAdminCategories(categoriesResponse.data.categories || [])
          setAdminDeliveries(deliveriesResponse.data.deliveries || [])
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

  useEffect(() => {
    if (!user) return undefined
    let active = true
    Promise.all([
      axios.get(`${API_BASE_URL}/api/notifications?limit=30`, { headers: getAuthHeaders() }),
      axios.get(`${API_BASE_URL}/api/notifications/unread-count`, { headers: getAuthHeaders() }),
    ]).then(([listResponse, countResponse]) => {
      if (!active) return
      setNotifications(listResponse.data.notifications || [])
      setUnreadNotificationCount(countResponse.data.unread || 0)
    }).catch((error) => {
      if (active) setOrderError(error.response?.data?.message || 'Unable to load notifications.')
    })
    return () => { active = false }
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
  const sellerAnalytics = useMemo(() => {
    const completedOrders = sellerOrders.filter((order) => order.status === 'delivered')
    const productSales = new Map()
    for (const order of completedOrders) {
      for (const item of order.items || []) {
        const current = productSales.get(item.productId) || { name: item.name, quantity: 0, total: 0 }
        current.quantity += item.quantity
        current.total += item.lineTotal
        productSales.set(item.productId, current)
      }
    }
    const openOrders = sellerOrders.filter((order) =>
      !['delivered', 'cancelled', 'rejected'].includes(order.status),
    )
    const statuses = sellerOrders.reduce((counts, order) => {
      counts[order.status] = (counts[order.status] || 0) + 1
      return counts
    }, {})
    const recentOrders = [...sellerOrders]
      .sort((left, right) => new Date(right.createdAt) - new Date(left.createdAt))
      .slice(0, 5)
    const topProducts = [...productSales.values()].sort((left, right) => right.quantity - left.quantity).slice(0, 5)
    return {
      completedOrders,
      openOrders,
      statuses,
      recentOrders,
      topProducts,
      salesTotal: completedOrders.reduce((total, order) => total + order.subtotal, 0),
      unitsSold: [...productSales.values()].reduce((total, product) => total + product.quantity, 0),
      lowStock: sellerProducts.filter((product) => product.stock <= 5).length,
    }
  }, [sellerOrders, sellerProducts])
  const adminSellers = adminUsers.filter((member) => member.role === 'seller')
  const adminAgents = adminUsers.filter((member) => member.role === 'delivery_agent')
  const adminPendingSellers = adminSellers.filter((seller) =>
    (seller.sellerStatus || (seller.verified ? 'approved' : 'pending')) === 'pending',
  )
  const filteredAdminUsers = adminUsers.filter((member) => {
    const matchesRole = adminUserRole === 'all' || member.role === adminUserRole
    const searchable = `${member.name} ${member.email} ${member.phone || ''}`.toLowerCase()
    return matchesRole && searchable.includes(adminUserSearch.trim().toLowerCase())
  })
  const filteredAdminOrders = adminOrders.filter((order) => {
    const matchesStatus = adminOrderStatus === 'all' || order.status === adminOrderStatus
    const searchable = `${order.id} ${order.customerId} ${order.sellerId} ${order.villageId}`.toLowerCase()
    return matchesStatus && searchable.includes(adminOrderSearch.trim().toLowerCase())
  })
  const agentPerformance = (agentId) => {
    const assignments = adminDeliveries.filter((delivery) => delivery.agentId === agentId)
    return {
      total: assignments.length,
      delivered: assignments.filter((delivery) => delivery.status === 'delivered').length,
      failed: assignments.filter((delivery) => delivery.status === 'failed').length,
    }
  }
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

    if (authMode === 'register' && authForm.password.length < 8) {
      setAuthMessage('Password must be at least 8 characters.')
      return
    }
    if (authForm.password.length > 72) {
      setAuthMessage('Password cannot be longer than 72 characters.')
      return
    }

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
      const message = authMode === 'register' && error.response?.status === 409
        ? 'An account with this email already exists. Sign in or use a different email.'
        : error.response?.data?.message || 'Authentication failed.'
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

  const handleSellerProductSave = async (event) => {
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
        stock: Number(effectiveProductDraft.stock),
        description: effectiveProductDraft.description,
        available: effectiveProductDraft.available && Number(effectiveProductDraft.stock) > 0,
      }

      if (productEditId) {
        const existing = sellerProducts.find((product) => product.id === productEditId)
        await axios.put(`${API_BASE_URL}/api/products/${productEditId}`, {
          ...payload,
          rating: existing?.rating || 0,
        }, { headers: getAuthHeaders() })
        setDashboardMessage('Product updated successfully.')
      } else {
        await axios.post(`${API_BASE_URL}/api/products`, payload, { headers: getAuthHeaders() })
        setDashboardMessage('Product listed successfully.')
      }
      setProductEditId('')
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
      await refreshSellerProducts()
    } catch (error) {
      setDashboardMessage(error.response?.data?.message || 'Unable to save this product.')
    }
  }

  const refreshSellerProducts = async () => {
    const response = await axios.get(`${API_BASE_URL}/api/sellers/${user.id}/products`, {
      headers: getAuthHeaders(),
    })
    setSellerProducts(response.data.products || [])
  }

  const handleSellerProductEdit = (product) => {
    setProductEditId(product.id)
    setProductDraft({
      name: product.name,
      category: product.category,
      price: String(product.price),
      stock: String(product.stock),
      unit: product.unit,
      villageId: product.villageId,
      village: product.village,
      seller: user.name,
      available: product.available,
      description: product.description,
    })
    setSellerSection('Products')
    setDashboardMessage('')
  }

  const handleSellerProductUpdate = async (product, updates) => {
    setDashboardMessage('')
    try {
      const nextStock = updates.stock ?? product.stock
      const nextAvailable = (updates.available ?? product.available) && nextStock > 0
      await axios.put(`${API_BASE_URL}/api/products/${product.id}`, {
        name: product.name,
        category: product.category,
        price: updates.price ?? product.price,
        rating: product.rating,
        unit: product.unit,
        villageId: product.villageId,
        village: product.village,
        available: nextAvailable,
        stock: nextStock,
        description: product.description,
      }, { headers: getAuthHeaders() })
      await refreshSellerProducts()
      setDashboardMessage('Product inventory updated.')
    } catch (error) {
      setDashboardMessage(error.response?.data?.message || 'Unable to update this product.')
    }
  }

  const handleInventorySave = (event, product) => {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    handleSellerProductUpdate(product, {
      price: Number(formData.get('price')),
      stock: Number(formData.get('stock')),
    })
  }

  const handleSellerProductDelete = async (product) => {
    if (!window.confirm(`Delete ${product.name} from your listings?`)) return
    setDashboardMessage('')
    try {
      await axios.delete(`${API_BASE_URL}/api/products/${product.id}`, { headers: getAuthHeaders() })
      await refreshSellerProducts()
      setDashboardMessage('Product deleted.')
    } catch (error) {
      setDashboardMessage(error.response?.data?.message || 'Unable to delete this product.')
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
      setDashboardMessage('Order status updated.')
    } catch (error) {
      setOrderError(error.response?.data?.message || 'Unable to update this order.')
      try {
        const response = await axios.get(`${API_BASE_URL}/api/seller/orders`, { headers: getAuthHeaders() })
        setSellerOrders(response.data.orders || [])
      } catch {}
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
      try {
        const response = await axios.get(`${API_BASE_URL}/api/orders`, { headers: getAuthHeaders() })
        setCustomerOrders(response.data.orders || [])
      } catch {}
    }
  }

  const toggleCustomerOrderDetails = async (order) => {
    if (expandedCustomerOrderId === order.id) {
      setExpandedCustomerOrderId('')
      return
    }
    setExpandedCustomerOrderId(order.id)
    setOrderError('')
    try {
      const sellerRequest = order.sellerId
        ? axios.get(`${API_BASE_URL}/api/sellers/${order.sellerId}`)
        : Promise.resolve({ data: { seller: null } })
      const [trackingResponse, reviewsResponse, sellerResponse] = await Promise.all([
        axios.get(`${API_BASE_URL}/api/orders/${order.id}/tracking`, { headers: getAuthHeaders() }),
        axios.get(`${API_BASE_URL}/api/orders/${order.id}/reviews`, { headers: getAuthHeaders() }),
        sellerRequest,
      ])
      setOrderTracking((current) => ({ ...current, [order.id]: trackingResponse.data.tracking }))
      setOrderReviews((current) => ({ ...current, [order.id]: reviewsResponse.data.reviews || [] }))
      setOrderSellerProfiles((current) => ({ ...current, [order.id]: sellerResponse.data.seller || null }))
    } catch (error) {
      setOrderError(error.response?.data?.message || 'Unable to load order details.')
    }
  }

  const submitOrderItemReview = async (event, order, item) => {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    setReviewMessage('')
    try {
      await axios.post(`${API_BASE_URL}/api/reviews`, {
        orderId: order.id,
        productId: item.productId,
        productRating: Number(formData.get('productRating')),
        sellerRating: Number(formData.get('sellerRating')),
        comment: formData.get('comment'),
      }, { headers: getAuthHeaders() })
      const response = await axios.get(`${API_BASE_URL}/api/orders/${order.id}/reviews`, {
        headers: getAuthHeaders(),
      })
      setOrderReviews((current) => ({ ...current, [order.id]: response.data.reviews || [] }))
      setReviewMessage('Your review has been submitted.')
    } catch (error) {
      setReviewMessage(error.response?.data?.message || 'Unable to submit this review.')
    }
  }

  const refreshAdminDashboard = async () => {
    const headers = { headers: getAuthHeaders() }
    const [stats, users, productsResponse, ordersResponse, villagesResponse, categoriesResponse, deliveriesResponse] = await Promise.all([
      axios.get(`${API_BASE_URL}/api/admin/analytics`, headers),
      axios.get(`${API_BASE_URL}/api/admin/users`, headers),
      axios.get(`${API_BASE_URL}/api/admin/products`, headers),
      axios.get(`${API_BASE_URL}/api/admin/orders`, headers),
      axios.get(`${API_BASE_URL}/api/admin/villages`, headers),
      axios.get(`${API_BASE_URL}/api/admin/categories`, headers),
      axios.get(`${API_BASE_URL}/api/deliveries`, headers),
    ])
    setAdminStats(stats.data.stats || {})
    setAdminUsers(users.data.users || [])
    setAdminProducts(productsResponse.data.products || [])
    setAdminOrders(ordersResponse.data.orders || [])
    setAdminVillages(villagesResponse.data.villages || [])
    setAdminCategories(categoriesResponse.data.categories || [])
    setAdminDeliveries(deliveriesResponse.data.deliveries || [])
  }

  const runAdminAction = async (request, successMessage) => {
    setAdminMessage('')
    try {
      await request()
      await refreshAdminDashboard()
      setAdminMessage(successMessage)
    } catch (error) {
      setAdminMessage(error.response?.data?.message || 'Unable to complete this admin action.')
    }
  }

  const handleAdminUserActive = (member) => runAdminAction(() => axios.patch(
    `${API_BASE_URL}/api/admin/${member.role === 'delivery_agent' ? 'agents' : 'users'}/${member.id}/active`,
    { active: member.active === false }, { headers: getAuthHeaders() },
  ), `Account ${member.active === false ? 'activated' : 'deactivated'}.`)

  const handleAdminSellerAction = (sellerId, action) => runAdminAction(() => axios.patch(
    `${API_BASE_URL}/api/admin/sellers/${sellerId}/status`, { action }, { headers: getAuthHeaders() },
  ), `Seller ${action}d.`)

  const handleAdminProductAvailability = (product) => runAdminAction(() => axios.patch(
    `${API_BASE_URL}/api/admin/products/${product.id}/availability`,
    { available: !product.available }, { headers: getAuthHeaders() },
  ), `Product ${product.available ? 'deactivated' : 'activated'}.`)

  const handleAdminVillageSubmit = (event) => {
    event.preventDefault()
    const request = adminVillageEditId
      ? () => axios.put(`${API_BASE_URL}/api/admin/villages/${adminVillageEditId}`, adminVillageDraft, { headers: getAuthHeaders() })
      : () => axios.post(`${API_BASE_URL}/api/admin/villages`, adminVillageDraft, { headers: getAuthHeaders() })
    runAdminAction(request, adminVillageEditId ? 'Village updated.' : 'Village created.')
      .then(() => {
        setAdminVillageDraft({ name: '', district: '', taluk: '', state: '' })
        setAdminVillageEditId('')
      })
  }

  const handleAdminCategorySubmit = (event) => {
    event.preventDefault()
    const request = adminCategoryEditId
      ? () => axios.put(`${API_BASE_URL}/api/admin/categories/${adminCategoryEditId}`, adminCategoryDraft, { headers: getAuthHeaders() })
      : () => axios.post(`${API_BASE_URL}/api/admin/categories`, adminCategoryDraft, { headers: getAuthHeaders() })
    runAdminAction(request, adminCategoryEditId ? 'Category updated.' : 'Category created.')
      .then(() => {
        setAdminCategoryDraft({ name: '' })
        setAdminCategoryEditId('')
      })
  }

  const handleAdminVillageActive = (village) => runAdminAction(() => axios.patch(
    `${API_BASE_URL}/api/admin/villages/${village.id}/active`,
    { active: village.active === false }, { headers: getAuthHeaders() },
  ), `Village ${village.active === false ? 'activated' : 'deactivated'}.`)

  const handleAdminCategoryActive = (category) => runAdminAction(() => axios.patch(
    `${API_BASE_URL}/api/admin/categories/${category.id}/active`,
    { active: category.active === false }, { headers: getAuthHeaders() },
  ), `Category ${category.active === false ? 'activated' : 'deactivated'}.`)

  const refreshNotifications = async () => {
    const [listResponse, countResponse] = await Promise.all([
      axios.get(`${API_BASE_URL}/api/notifications?limit=30`, { headers: getAuthHeaders() }),
      axios.get(`${API_BASE_URL}/api/notifications/unread-count`, { headers: getAuthHeaders() }),
    ])
    setNotifications(listResponse.data.notifications || [])
    setUnreadNotificationCount(countResponse.data.unread || 0)
  }

  const handleMarkNotificationRead = async (notification) => {
    if (!notification.read) {
      try {
        await axios.patch(`${API_BASE_URL}/api/notifications/${notification.id}/read`, {}, {
          headers: getAuthHeaders(),
        })
        setNotifications((current) => current.map((item) =>
          item.id === notification.id ? { ...item, read: true } : item,
        ))
        setUnreadNotificationCount((current) => Math.max(0, current - 1))
      } catch (error) {
        setOrderError(error.response?.data?.message || 'Unable to mark notification as read.')
      }
    }
    if (notification.type === 'seller_verification') setAdminSection('Seller Verification')
    if (notification.type === 'new_order' || notification.type === 'low_stock') setSellerSection('Orders')
  }

  const handleSellerVerificationRequest = async () => {
    try {
      await axios.post(`${API_BASE_URL}/api/seller/verification-request`, {}, { headers: getAuthHeaders() })
      setDashboardMessage('Verification request sent to administrators.')
      const response = await axios.get(`${API_BASE_URL}/api/auth/me`, { headers: getAuthHeaders() })
      setUser(response.data.user)
      localStorage.setItem('villageconnect_user', JSON.stringify(response.data.user))
    } catch (error) {
      setDashboardMessage(error.response?.data?.message || 'Unable to request seller verification.')
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
    setExpandedCustomerOrderId('')
    setOrderSellerProfiles({})
    setOrderReviews({})
    setReviewMessage('')
    setOrderError('')
    setSellerProducts([])
    setAdminStats({ users: 0, sellers: 0, deliveryAgents: 0, products: 0, orders: 0, revenue: 0 })
    setAdminUsers([])
    setAdminProducts([])
    setAdminOrders([])
    setAdminVillages([])
    setAdminCategories([])
    setAdminDeliveries([])
    setAdminSection('Overview')
    setAdminMessage('')
    setAgentDeliveries([])
    setNotifications([])
    setUnreadNotificationCount(0)
    setNotificationsOpen(false)
  }

  const renderAdminBars = (title, rows, labelKey, valueKey) => {
    const max = Math.max(0, ...rows.map((row) => Number(row[valueKey] || 0)))
    return (
      <div className="dashboard-card admin-chart-card">
        <h3>{title}</h3>
        {rows.length === 0 ? <p className="auth-message">No completed sales yet.</p> : (
          <div className="admin-bar-list">
            {rows.map((row, index) => {
              const value = Number(row[valueKey] || 0)
              return <div className="admin-bar-row" key={`${row[labelKey]}-${index}`}>
                <div><span>{row[labelKey] || 'Other'}</span><strong>{formatCurrency(value)}</strong></div>
                <div className="admin-bar-track"><span style={{ width: `${max > 0 ? Math.max(3, (value / max) * 100) : 0}%` }} /></div>
              </div>
            })}
          </div>
        )}
      </div>
    )
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
          {user && (
            <div className="notification-menu">
              <button type="button" className="notification-button" aria-label={`Notifications, ${unreadNotificationCount} unread`}
                aria-expanded={notificationsOpen}
                onClick={async () => {
                  const opening = !notificationsOpen
                  setNotificationsOpen(opening)
                  if (opening) {
                    try { await refreshNotifications() } catch (error) {
                      setOrderError(error.response?.data?.message || 'Unable to refresh notifications.')
                    }
                  }
                }}>
                <Bell size={18} />
                <span>Notifications</span>
                {unreadNotificationCount > 0 && <span className="notification-count">{unreadNotificationCount > 99 ? '99+' : unreadNotificationCount}</span>}
              </button>
              {notificationsOpen && <div className="notification-popover" role="region" aria-label="Notifications">
                <div className="notification-popover-header"><strong>Notifications</strong><span>{unreadNotificationCount} unread</span></div>
                {notifications.length === 0 ? <p className="auth-message">You are all caught up.</p> : <div className="notification-list">
                  {notifications.map((notification) => <button type="button" className={`notification-item ${notification.read ? 'read' : 'unread'}`} key={notification.id}
                    onClick={() => handleMarkNotificationRead(notification)}>
                    <strong>{notification.title}</strong><span>{notification.message}</span><time>{new Date(notification.createdAt).toLocaleString('en-IN')}</time>
                  </button>)}
                </div>}
              </div>}
            </div>
          )}
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
            <>
              <nav className="seller-dashboard-tabs" aria-label="Seller dashboard sections">
                {['Overview', 'Products', 'Inventory', 'Orders', 'Sales', 'Analytics', 'Profile'].map((section) => (
                  <button key={section} type="button" className={sellerSection === section ? 'active' : ''}
                    aria-current={sellerSection === section ? 'page' : undefined}
                    onClick={() => setSellerSection(section)}>{section}</button>
                ))}
              </nav>
              {dashboardMessage && <p className="seller-dashboard-message" role="status">{dashboardMessage}</p>}
              {orderError && <p className="cart-error" role="alert">{orderError}</p>}

              {sellerSection === 'Overview' && (
                <div className="dashboard-grid seller-dashboard-grid">
                  <div className="stat-card"><span>Active listings</span><strong>{sellerProducts.filter((product) => product.available && product.stock > 0).length}</strong></div>
                  <div className="stat-card"><span>Units in stock</span><strong>{sellerProducts.reduce((sum, product) => sum + product.stock, 0)}</strong></div>
                  <div className="stat-card"><span>Open orders</span><strong>{sellerAnalytics.openOrders.length}</strong></div>
                  <div className="stat-card"><span>Completed sales</span><strong>{formatCurrency(sellerAnalytics.salesTotal)}</strong></div>
                  <div className="dashboard-card wide-card">
                    <div className="panel-header"><h3>Recent orders</h3><button type="button" className="text-button" onClick={() => setSellerSection('Orders')}>All orders <ChevronRight size={15} /></button></div>
                    <div className="mini-list">
                      {sellerAnalytics.recentOrders.length === 0 ? <p className="auth-message">Incoming orders will appear here.</p> : sellerAnalytics.recentOrders.map((order) => (
                        <div className="mini-list-item" key={order.id}>
                          <div><strong>Order {order.id}</strong><span>{order.items?.length || 0} items · {order.status.replaceAll('_', ' ')}</span></div>
                          <strong>{formatCurrency(order.total)}</strong>
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="dashboard-card">
                    <div className="panel-header"><h3>Inventory attention</h3><button type="button" className="text-button" onClick={() => setSellerSection('Inventory')}>Manage <ChevronRight size={15} /></button></div>
                    <p className="seller-summary-number">{sellerAnalytics.lowStock}</p>
                    <span className="seller-summary-caption">listings at 5 units or fewer</span>
                  </div>
                </div>
              )}

              {sellerSection === 'Products' && (
                <div className="dashboard-grid">
                  <div className="dashboard-card">
                    <h3>{productEditId ? 'Edit product' : 'Create a product'}</h3>
                    <form className="dashboard-form" onSubmit={handleSellerProductSave}>
                      <input name="name" value={effectiveProductDraft.name} onChange={handleProductDraftChange} placeholder="Product name" required />
                      <select name="category" value={effectiveProductDraft.category} onChange={handleProductDraftChange} required>
                        {!effectiveProductDraft.category && <option value="">Choose a category</option>}
                        {categories.map((category) => <option key={category.id} value={category.name}>{category.name}</option>)}
                      </select>
                      <label>Price (INR)<input name="price" type="number" min="0.01" step="0.01" value={effectiveProductDraft.price} onChange={handleProductDraftChange} required /></label>
                      <label>Stock<input name="stock" type="number" min="0" step="1" value={effectiveProductDraft.stock} onChange={handleProductDraftChange} required /></label>
                      <input name="unit" value={effectiveProductDraft.unit} onChange={handleProductDraftChange} placeholder="Unit (kg/dozen)" required />
                      <select name="villageId" value={effectiveProductDraft.villageId} onChange={handleProductDraftChange} required>
                        <option value="">Choose a village</option>
                        {villages.map((village) => <option key={village.id} value={village.id}>{village.name}</option>)}
                      </select>
                      <textarea name="description" value={effectiveProductDraft.description} onChange={handleProductDraftChange} placeholder="Product description" required />
                      <label className="seller-availability-control"><input name="available" type="checkbox" checked={effectiveProductDraft.available} onChange={handleProductDraftChange} />Available now</label>
                      <button type="submit" className="primary-button full-width">{productEditId ? 'Save product' : 'List product'}</button>
                      {productEditId && <button type="button" className="secondary-button full-width" onClick={() => { setProductEditId(''); setProductDraft({ ...productDraft, name: '', price: '', stock: '', description: '' }) }}>Cancel edit</button>}
                    </form>
                  </div>
                  <div className="dashboard-card wide-card">
                    <h3>Your products</h3>
                    <div className="mini-list">
                      {sellerProducts.length === 0 ? <p className="auth-message">No products listed yet.</p> : sellerProducts.map((product) => (
                        <div className="mini-list-item seller-product-row" key={product.id}>
                          <div><strong>{product.name}</strong><span>{product.village} · {product.category} · {formatCurrency(product.price)} / {product.unit}</span><span>{product.stock} in stock · {product.available ? 'available' : 'inactive'}</span></div>
                          <div className="seller-row-actions">
                            <button type="button" className="secondary-button small" onClick={() => handleSellerProductEdit(product)}>Edit</button>
                            <button type="button" className="secondary-button small" onClick={() => handleSellerProductUpdate(product, { available: !product.available })}>{product.available ? 'Deactivate' : 'Activate'}</button>
                            <button type="button" className="danger-button" onClick={() => handleSellerProductDelete(product)} aria-label={`Delete ${product.name}`}>Delete</button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              )}

              {sellerSection === 'Inventory' && (
                <div className="dashboard-card">
                  <div className="panel-header"><h3>Inventory and pricing</h3><span>{sellerProducts.length} listings</span></div>
                  {sellerProducts.length === 0 ? <p className="auth-message">Create a product to start managing inventory.</p> : (
                    <div className="mini-list">
                      {sellerProducts.map((product) => (
                        <div className="mini-list-item inventory-row" key={product.id}>
                          <div className="inventory-product"><strong>{product.name}</strong><span>{product.unit} · {product.stock} currently in stock</span></div>
                          <form className="inventory-update-form" onSubmit={(event) => handleInventorySave(event, product)}>
                            <label>Price<input name="price" type="number" min="0.01" step="0.01" defaultValue={product.price} required /></label>
                            <label>Stock<input name="stock" type="number" min="0" step="1" defaultValue={product.stock} required /></label>
                            <button className="primary-button small" type="submit">Save</button>
                          </form>
                          <button type="button" className={`availability-toggle ${product.available ? 'is-on' : ''}`} aria-pressed={product.available}
                            onClick={() => handleSellerProductUpdate(product, { available: !product.available })}>
                            {product.available ? 'Available' : 'Unavailable'}
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {sellerSection === 'Orders' && (
                <div className="dashboard-card">
                  <div className="panel-header"><h3>Incoming orders</h3><span>{sellerOrders.length} total</span></div>
                  {orderError && <p className="cart-error" role="alert">{orderError}</p>}
                  <div className="mini-list">
                    {sellerOrders.length === 0 ? <p className="auth-message">No customer orders yet.</p> : sellerOrders.map((order) => {
                      const actions = {
                        pending: [['confirmed', 'Accept'], ['rejected', 'Reject']],
                        confirmed: [['preparing', 'Prepare order']],
                        preparing: [['packed', 'Mark packed']],
                        packed: [['ready_for_pickup', order.fulfillmentType === 'pickup' ? 'Ready for pickup' : 'Ready for delivery']],
                      }[order.status] || []
                      const expanded = selectedSellerOrderId === order.id
                      return (
                        <article className="seller-order" key={order.id}>
                          <div className="mini-list-item order-list-item">
                            <div><strong>Order {order.id}</strong><span>{order.items?.map((item) => `${item.name} × ${item.quantity}`).join(', ')}</span><span>{order.fulfillmentType} · {new Date(order.createdAt).toLocaleString()}</span></div>
                            <div className="order-actions"><span className="status-pill">{order.status.replaceAll('_', ' ')}</span><strong>{formatCurrency(order.total)}</strong><button type="button" className="secondary-button small" aria-expanded={expanded} onClick={() => setSelectedSellerOrderId(expanded ? '' : order.id)}>{expanded ? 'Hide details' : 'Details'}</button>{actions.map(([status, label]) => <button key={status} type="button" className={status === 'rejected' ? 'secondary-button small' : 'primary-button small'} onClick={() => handleSellerOrderStatus(order.id, status)}>{label}</button>)}</div>
                          </div>
                          {expanded && <div className="seller-order-details"><div><strong>Items</strong>{order.items?.map((item) => <span key={item.productId}>{item.name} × {item.quantity} {item.unit} · {formatCurrency(item.lineTotal)}</span>)}</div><div><strong>Fulfillment</strong><span>{order.fulfillmentType === 'delivery' ? order.address || 'Delivery address not provided' : 'Customer pickup'}</span><span>Village: {villages.find((village) => village.id === order.villageId)?.name || order.villageId}</span></div><div><strong>Payment</strong><span>{order.payment?.method?.replaceAll('_', ' ')} · {order.payment?.status}</span></div><div><strong>Status history</strong>{order.statusHistory?.map((event, index) => <span key={`${event.status}-${event.createdAt}-${index}`}>{event.status.replaceAll('_', ' ')} · {new Date(event.createdAt).toLocaleString()}</span>)}</div></div>}
                        </article>
                      )
                    })}
                  </div>
                </div>
              )}

              {sellerSection === 'Sales' && (
                <div className="dashboard-grid">
                  <div className="stat-card"><span>Completed sales</span><strong>{formatCurrency(sellerAnalytics.salesTotal)}</strong></div>
                  <div className="stat-card"><span>Delivered orders</span><strong>{sellerAnalytics.completedOrders.length}</strong></div>
                  <div className="stat-card"><span>Units sold</span><strong>{sellerAnalytics.unitsSold}</strong></div>
                  <div className="dashboard-card wide-card"><h3>Sales history</h3><div className="mini-list">
                    {sellerOrders.length === 0 ? <p className="auth-message">Sales and order history will appear here.</p> : [...sellerOrders].sort((left, right) => new Date(right.createdAt) - new Date(left.createdAt)).map((order) => (
                      <div className="mini-list-item" key={order.id}><div><strong>Order {order.id}</strong><span>{new Date(order.createdAt).toLocaleDateString()} · {order.items?.map((item) => `${item.name} × ${item.quantity}`).join(', ')}</span></div><div className="order-actions"><span className="status-pill">{order.status.replaceAll('_', ' ')}</span><strong>{formatCurrency(order.total)}</strong></div></div>
                    ))}
                  </div></div>
                </div>
              )}

              {sellerSection === 'Analytics' && (
                <div className="dashboard-grid">
                  <div className="dashboard-card"><h3>Orders by status</h3><div className="mini-list">{Object.entries(sellerAnalytics.statuses).length === 0 ? <span className="auth-message">No order data yet.</span> : Object.entries(sellerAnalytics.statuses).map(([status, count]) => <div className="mini-list-item" key={status}><strong>{status.replaceAll('_', ' ')}</strong><span>{count}</span></div>)}</div></div>
                  <div className="dashboard-card"><h3>Top products</h3><div className="mini-list">{sellerAnalytics.topProducts.length === 0 ? <span className="auth-message">Completed product sales will appear here.</span> : sellerAnalytics.topProducts.map((product) => <div className="mini-list-item" key={product.name}><div><strong>{product.name}</strong><span>{product.quantity} units delivered</span></div><strong>{formatCurrency(product.total)}</strong></div>)}</div></div>
                  <div className="stat-card"><span>Open orders</span><strong>{sellerAnalytics.openOrders.length}</strong></div>
                  <div className="stat-card"><span>Low-stock products</span><strong>{sellerAnalytics.lowStock}</strong></div>
                </div>
              )}

              {sellerSection === 'Profile' && (
                <div className="dashboard-card seller-profile-panel">
                  <div className="panel-header"><h3>Seller profile</h3><span className={`verification-badge ${user.verified ? 'verified' : 'unverified'}`}><ShieldCheck size={16} />{user.verified ? 'Verified seller' : 'Verification pending'}</span></div>
                  <dl className="seller-profile-details"><div><dt>Business / seller name</dt><dd>{user.name}</dd></div><div><dt>Email</dt><dd>{user.email}</dd></div><div><dt>Phone</dt><dd>{user.phone || 'Not provided'}</dd></div><div><dt>Home village</dt><dd>{villages.find((village) => village.id === user.villageId)?.name || 'Not provided'}</dd></div><div><dt>Account status</dt><dd>{user.verified ? 'Verified and approved to sell' : 'Verification required before selling'}</dd></div></dl>
                  {!user.verified && <button type="button" className="primary-button small" onClick={handleSellerVerificationRequest} disabled={user.sellerStatus === 'pending'}>{user.sellerStatus === 'pending' ? 'Verification requested' : 'Request verification'}</button>}
                  {dashboardMessage && <p className="seller-dashboard-message" role="status">{dashboardMessage}</p>}
                </div>
              )}
            </>
          )}

          {user.role === 'customer' && (
            <div className="dashboard-card customer-orders-panel">
              <div className="panel-header"><h3>My orders</h3><span>{customerOrders.length} orders</span></div>
              {orderError && <p className="cart-error" role="alert">{orderError}</p>}
              <div className="customer-order-list">
                {customerOrders.length === 0 ? (
                  <p className="auth-message">Your placed orders will appear here.</p>
                ) : customerOrders.map((order) => {
                  const tracking = orderTracking[order.id]
                  const seller = orderSellerProfiles[order.id]
                  const reviews = orderReviews[order.id] || []
                  const expanded = expandedCustomerOrderId === order.id
                  return (
                    <article className="customer-order" key={order.id}>
                      <div className="customer-order-heading">
                        <div>
                          <strong>Order {order.id}</strong>
                          <span>{order.items?.map((item) => `${item.name} × ${item.quantity}`).join(', ')}</span>
                          <span>{new Date(order.createdAt).toLocaleString('en-IN')} · {order.fulfillmentType.replaceAll('_', ' ')}</span>
                        </div>
                        <div className="customer-order-actions">
                          <span className="status-pill">{order.status.replaceAll('_', ' ')}</span>
                          <strong>{formatCurrency(order.total)}</strong>
                          <button type="button" className="secondary-button small" aria-expanded={expanded}
                            onClick={() => toggleCustomerOrderDetails(order)}>{expanded ? 'Hide details' : 'Details & tracking'}</button>
                          {['pending', 'confirmed'].includes(order.status) && <button type="button" className="secondary-button small" onClick={() => cancelCustomerOrder(order.id)}>Cancel order</button>}
                        </div>
                      </div>

                      {expanded && (
                        <div className="customer-order-details">
                          <section className="customer-order-block customer-timeline-block">
                            <h4>Order timeline</h4>
                            {['cancelled', 'rejected'].includes(order.status) && <p className="order-terminal-state">This order was {order.status}.</p>}
                            <ol className="customer-timeline">
                              {customerTimeline(order, tracking).map((step) => (
                                <li key={step.label} className={step.complete ? 'complete' : ''}>
                                  <span className="timeline-marker" aria-hidden="true" />
                                  <strong>{step.label}</strong>
                                  {step.timestamp && <time>{new Date(step.timestamp).toLocaleString('en-IN')}</time>}
                                </li>
                              ))}
                            </ol>
                          </section>

                          <div className="customer-detail-columns">
                            <section className="customer-order-block">
                              <h4>Products</h4>
                              {order.items?.map((item) => (
                                <div className="customer-product-detail" key={item.productId}>
                                  <strong>{item.name}</strong>
                                  <span>{item.quantity} × {formatCurrency(item.price)} / {item.unit}</span>
                                  <span>Line total: {formatCurrency(item.lineTotal)}</span>
                                </div>
                              ))}
                            </section>
                            <section className="customer-order-block">
                              <h4>Seller</h4>
                              <strong>{seller?.name || order.items?.[0]?.seller || 'VillageConnect seller'}</strong>
                              {seller && <span>{villages.find((village) => village.id === seller.villageId)?.name || 'Village not listed'}</span>}
                              {seller?.ratingCount > 0 && <span><Star size={14} fill="currentColor" /> {seller.rating.toFixed(1)} · {seller.ratingCount} reviews</span>}
                            </section>
                            <section className="customer-order-block">
                              <h4>Payment</h4>
                              <span>{order.payment?.method?.replaceAll('_', ' ') || 'Payment method not recorded'}</span>
                              <span>{order.payment?.status || 'Status unavailable'}</span>
                              <strong>{formatCurrency(order.payment?.amount ?? order.total)}</strong>
                            </section>
                            <section className="customer-order-block">
                              <h4>Delivery</h4>
                              <span>{order.fulfillmentType === 'delivery' ? order.address || 'Delivery address not provided' : 'Pickup from seller'}</span>
                              <span>Delivery fee: {formatCurrency(order.deliveryFee || 0)}</span>
                              {tracking?.delivery && <span>Delivery status: {tracking.delivery.status.replaceAll('_', ' ')}</span>}
                              {tracking?.delivery?.updatedAt && <time>Updated {new Date(tracking.delivery.updatedAt).toLocaleString('en-IN')}</time>}
                              {tracking?.delivery?.history?.map((event, index) => <span className="delivery-history-event" key={`${event.status}-${event.createdAt}-${index}`}>{event.status.replaceAll('_', ' ')} · {new Date(event.createdAt).toLocaleString('en-IN')}</span>)}
                            </section>
                          </div>

                          {order.status === 'delivered' && (
                            <section className="customer-order-block customer-review-section">
                              <h4>Rate your purchase</h4>
                              {order.items?.map((item) => {
                                const review = reviews.find((entry) => entry.productId === item.productId)
                                return review ? (
                                  <div className="submitted-review" key={item.productId}>
                                    <strong>{item.name}</strong>
                                    <span>Product {review.productRating}/5 · Seller {review.sellerRating}/5</span>
                                    {review.comment && <p>{review.comment}</p>}
                                  </div>
                                ) : (
                                  <form className="customer-review-form" key={item.productId} onSubmit={(event) => submitOrderItemReview(event, order, item)}>
                                    <strong>{item.name}</strong>
                                    <label>Product rating<select name="productRating" defaultValue="5">{[5, 4, 3, 2, 1].map((rating) => <option key={rating} value={rating}>{rating} stars</option>)}</select></label>
                                    <label>Seller rating<select name="sellerRating" defaultValue="5">{[5, 4, 3, 2, 1].map((rating) => <option key={rating} value={rating}>{rating} stars</option>)}</select></label>
                                    <label className="review-comment">Review<textarea name="comment" maxLength="1000" placeholder="Share a few details about the product and service" /></label>
                                    <button type="submit" className="primary-button small">Submit review</button>
                                  </form>
                                )
                              })}
                              {reviewMessage && <p className="seller-dashboard-message" role="status">{reviewMessage}</p>}
                            </section>
                          )}
                        </div>
                      )}
                    </article>
                  )
                })}
              </div>
            </div>
          )}

          {user.role === 'admin' && (
            <>
              <nav className="seller-dashboard-tabs admin-dashboard-tabs" aria-label="Admin dashboard sections">
                {['Overview', 'Users', 'Sellers', 'Delivery Agents', 'Villages', 'Categories', 'Products', 'Orders', 'Seller Verification', 'Analytics'].map((section) => (
                  <button key={section} type="button" className={adminSection === section ? 'active' : ''}
                    aria-current={adminSection === section ? 'page' : undefined}
                    onClick={() => setAdminSection(section)}>{section}</button>
                ))}
              </nav>
              {adminMessage && <p className="seller-dashboard-message" role="status">{adminMessage}</p>}

              {adminSection === 'Overview' && (
                <div className="dashboard-grid admin-stats-grid">
                  <div className="stat-card"><span>Total users</span><strong>{adminStats.users || adminUsers.length}</strong></div>
                  <div className="stat-card"><span>Sellers</span><strong>{adminStats.sellers || adminSellers.length}</strong></div>
                  <div className="stat-card"><span>Delivery agents</span><strong>{adminStats.deliveryAgents || adminAgents.length}</strong></div>
                  <div className="stat-card"><span>Products</span><strong>{adminStats.products || adminProducts.length}</strong></div>
                  <div className="stat-card"><span>Orders</span><strong>{adminStats.orders || adminOrders.length}</strong></div>
                  <div className="stat-card"><span>Revenue</span><strong>{formatCurrency(Number(adminStats.revenue || 0))}</strong></div>
                  <div className="dashboard-card wide-card">
                    <div className="panel-header"><h3>Seller verification queue</h3><button type="button" className="text-button" onClick={() => setAdminSection('Seller Verification')}>Review sellers <ChevronRight size={15} /></button></div>
                    <p>{adminPendingSellers.length} sellers awaiting a decision</p>
                  </div>
                  <div className="dashboard-card">
                    <div className="panel-header"><h3>Order activity</h3><button type="button" className="text-button" onClick={() => setAdminSection('Orders')}>Investigate <ChevronRight size={15} /></button></div>
                    <strong>{adminOrders.filter((order) => !['delivered', 'cancelled', 'rejected'].includes(order.status)).length} open orders</strong>
                  </div>
                </div>
              )}

              {adminSection === 'Users' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Users</h3><span>{filteredAdminUsers.length} accounts</span></div>
                  <div className="admin-toolbar">
                    <input value={adminUserSearch} onChange={(event) => setAdminUserSearch(event.target.value)} placeholder="Search name, email, phone" aria-label="Search users" />
                    <select value={adminUserRole} onChange={(event) => setAdminUserRole(event.target.value)} aria-label="Filter users by role">
                      <option value="all">All roles</option><option value="customer">Customers</option><option value="seller">Sellers</option><option value="delivery_agent">Delivery agents</option><option value="admin">Admins</option>
                    </select>
                  </div>
                  <div className="admin-record-list">
                    {filteredAdminUsers.map((member) => (
                      <div className="admin-record-row" key={member.id}>
                        <div><strong>{member.name}</strong><span>{member.email} · {member.phone || 'No phone'}</span></div>
                        <span className="status-pill">{member.role.replaceAll('_', ' ')}</span>
                        <span className={`admin-state ${member.active === false ? 'inactive' : 'active'}`}>{member.active === false ? 'Inactive' : 'Active'}</span>
                        {member.id !== user.id && <button type="button" className="secondary-button small" onClick={() => handleAdminUserActive(member)}>{member.active === false ? 'Activate' : 'Deactivate'}</button>}
                      </div>
                    ))}
                    {filteredAdminUsers.length === 0 && <p className="auth-message">No matching users.</p>}
                  </div>
                </div>
              )}

              {adminSection === 'Sellers' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Sellers</h3><span>{adminSellers.length} accounts</span></div>
                  <div className="admin-record-list">
                    {adminSellers.map((seller) => {
                      const status = seller.sellerStatus || (seller.verified ? 'approved' : 'pending')
                      return <div className="admin-record-row" key={seller.id}>
                        <div><strong>{seller.name}</strong><span>{seller.email} · {villages.find((village) => village.id === seller.villageId)?.name || 'Village not listed'}</span></div>
                        <span className="status-pill">{status}</span>
                        <span className={`admin-state ${seller.active === false ? 'inactive' : 'active'}`}>{seller.active === false ? 'Inactive' : 'Active'}</span>
                        <div className="admin-row-actions">
                          {status === 'pending' && <><button type="button" className="primary-button small" onClick={() => handleAdminSellerAction(seller.id, 'approve')}>Approve</button><button type="button" className="danger-button" onClick={() => handleAdminSellerAction(seller.id, 'reject')}>Reject</button></>}
                          {!seller.verified && status !== 'pending' && <button type="button" className="primary-button small" onClick={() => handleAdminSellerAction(seller.id, 'verify')}>Verify</button>}
                          {status === 'suspended' && <button type="button" className="primary-button small" onClick={() => handleAdminSellerAction(seller.id, 'verify')}>Reinstate</button>}
                          {status !== 'suspended' && status !== 'rejected' && <button type="button" className="secondary-button small" onClick={() => handleAdminSellerAction(seller.id, 'suspend')}>Suspend</button>}
                        </div>
                      </div>
                    })}
                  </div>
                </div>
              )}

              {adminSection === 'Delivery Agents' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Delivery agents</h3><span>{adminAgents.length} agents</span></div>
                  <div className="admin-record-list">
                    {adminAgents.map((agent) => {
                      const performance = agentPerformance(agent.id)
                      return <div className="admin-record-row" key={agent.id}>
                        <div><strong>{agent.name}</strong><span>{agent.email} · {villages.find((village) => village.id === agent.villageId)?.name || 'Village not listed'}</span></div>
                        <span>{performance.total} assigned</span><span>{performance.delivered} delivered · {performance.failed} failed</span>
                        <span className={`admin-state ${agent.active === false ? 'inactive' : 'active'}`}>{agent.active === false ? 'Inactive' : 'Active'}</span>
                        <button type="button" className="secondary-button small" onClick={() => handleAdminUserActive(agent)}>{agent.active === false ? 'Activate' : 'Deactivate'}</button>
                      </div>
                    })}
                  </div>
                </div>
              )}

              {adminSection === 'Villages' && (
                <div className="dashboard-grid admin-management-grid">
                  <div className="dashboard-card">
                    <h3>{adminVillageEditId ? 'Edit village' : 'Create village'}</h3>
                    <form className="dashboard-form admin-entity-form" onSubmit={handleAdminVillageSubmit}>
                      <input placeholder="Village name" value={adminVillageDraft.name} onChange={(event) => setAdminVillageDraft({ ...adminVillageDraft, name: event.target.value })} required />
                      <input placeholder="District" value={adminVillageDraft.district} onChange={(event) => setAdminVillageDraft({ ...adminVillageDraft, district: event.target.value })} required />
                      <input placeholder="Taluk" value={adminVillageDraft.taluk} onChange={(event) => setAdminVillageDraft({ ...adminVillageDraft, taluk: event.target.value })} />
                      <input placeholder="State" value={adminVillageDraft.state} onChange={(event) => setAdminVillageDraft({ ...adminVillageDraft, state: event.target.value })} required />
                      <button type="submit" className="primary-button">{adminVillageEditId ? 'Save changes' : 'Create village'}</button>
                      {adminVillageEditId && <button type="button" className="secondary-button" onClick={() => { setAdminVillageEditId(''); setAdminVillageDraft({ name: '', district: '', taluk: '', state: '' }) }}>Cancel edit</button>}
                    </form>
                  </div>
                  <div className="dashboard-card wide-card">
                    <h3>Village hierarchy</h3><div className="admin-record-list">
                      {adminVillages.map((village) => <div className="admin-record-row" key={village.id}>
                        <div><strong>{village.name}</strong><span>{[village.taluk, village.district, village.state].filter(Boolean).join(' · ')}</span></div>
                        <span className={`admin-state ${village.active === false ? 'inactive' : 'active'}`}>{village.active === false ? 'Inactive' : 'Active'}</span>
                        <button type="button" className="secondary-button small" onClick={() => { setAdminVillageEditId(village.id); setAdminVillageDraft({ name: village.name, district: village.district, taluk: village.taluk || '', state: village.state }) }}>Edit</button>
                        <button type="button" className="secondary-button small" onClick={() => handleAdminVillageActive(village)}>{village.active === false ? 'Activate' : 'Deactivate'}</button>
                      </div>)}
                    </div>
                  </div>
                </div>
              )}

              {adminSection === 'Categories' && (
                <div className="dashboard-grid admin-management-grid">
                  <div className="dashboard-card">
                    <h3>{adminCategoryEditId ? 'Edit category' : 'Create category'}</h3>
                    <form className="dashboard-form admin-entity-form" onSubmit={handleAdminCategorySubmit}>
                      <input placeholder="Category name" value={adminCategoryDraft.name} onChange={(event) => setAdminCategoryDraft({ name: event.target.value })} required />
                      <button type="submit" className="primary-button">{adminCategoryEditId ? 'Save changes' : 'Create category'}</button>
                      {adminCategoryEditId && <button type="button" className="secondary-button" onClick={() => { setAdminCategoryEditId(''); setAdminCategoryDraft({ name: '' }) }}>Cancel edit</button>}
                    </form>
                  </div>
                  <div className="dashboard-card wide-card">
                    <h3>Categories</h3><div className="admin-record-list">
                      {adminCategories.map((category) => <div className="admin-record-row" key={category.id}>
                        <strong>{category.name}</strong><span className={`admin-state ${category.active === false ? 'inactive' : 'active'}`}>{category.active === false ? 'Inactive' : 'Active'}</span>
                        <button type="button" className="secondary-button small" onClick={() => { setAdminCategoryEditId(category.id); setAdminCategoryDraft({ name: category.name }) }}>Edit</button>
                        <button type="button" className="secondary-button small" onClick={() => handleAdminCategoryActive(category)}>{category.active === false ? 'Activate' : 'Deactivate'}</button>
                      </div>)}
                    </div>
                  </div>
                </div>
              )}

              {adminSection === 'Products' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Product moderation</h3><span>{adminProducts.length} listings</span></div>
                  <div className="admin-record-list">
                    {adminProducts.map((product) => <div className="admin-record-row" key={product.id}>
                      <div><strong>{product.name}</strong><span>{product.category} · {product.seller} · {product.village}</span></div>
                      <span>{formatCurrency(product.price)} · {product.stock} in stock</span>
                      <span className={`admin-state ${product.available && product.stock > 0 ? 'active' : 'inactive'}`}>{product.available && product.stock > 0 ? 'Available' : 'Inactive'}</span>
                      <button type="button" className="secondary-button small" onClick={() => handleAdminProductAvailability(product)}>{product.available ? 'Deactivate' : 'Activate'}</button>
                    </div>)}
                  </div>
                </div>
              )}

              {adminSection === 'Orders' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Orders</h3><span>{filteredAdminOrders.length} orders</span></div>
                  <div className="admin-toolbar">
                    <input value={adminOrderSearch} onChange={(event) => setAdminOrderSearch(event.target.value)} placeholder="Search order, customer, seller, village" aria-label="Search orders" />
                    <select value={adminOrderStatus} onChange={(event) => setAdminOrderStatus(event.target.value)} aria-label="Filter orders by status">
                      <option value="all">All statuses</option>{['pending', 'confirmed', 'preparing', 'packed', 'ready_for_pickup', 'out_for_delivery', 'delivered', 'cancelled', 'rejected'].map((status) => <option key={status} value={status}>{status.replaceAll('_', ' ')}</option>)}
                    </select>
                  </div>
                  <div className="admin-record-list">
                    {filteredAdminOrders.map((order) => {
                      const expanded = adminExpandedOrderId === order.id
                      return <article className="admin-order-record" key={order.id}>
                        <div className="admin-record-row">
                          <div><strong>Order {order.id}</strong><span>{order.customerId} · {order.sellerId} · {new Date(order.createdAt).toLocaleString('en-IN')}</span></div>
                          <span className="status-pill">{order.status.replaceAll('_', ' ')}</span><strong>{formatCurrency(order.total)}</strong>
                          <button type="button" className="secondary-button small" aria-expanded={expanded} onClick={() => setAdminExpandedOrderId(expanded ? '' : order.id)}>{expanded ? 'Close' : 'Investigate'}</button>
                        </div>
                        {expanded && <div className="admin-order-details"><div><strong>Items</strong>{order.items?.map((item) => <span key={item.productId}>{item.name} × {item.quantity} · {formatCurrency(item.lineTotal)}</span>)}</div><div><strong>Fulfillment</strong><span>{order.fulfillmentType} · {order.address || order.villageId}</span><span>Payment: {order.payment?.method} · {order.payment?.status}</span></div><div><strong>Status history</strong>{order.statusHistory?.map((event, index) => <span key={`${event.status}-${index}`}>{event.status.replaceAll('_', ' ')} · {new Date(event.createdAt).toLocaleString('en-IN')}</span>)}</div></div>}
                      </article>
                    })}
                    {filteredAdminOrders.length === 0 && <p className="auth-message">No matching orders.</p>}
                  </div>
                </div>
              )}

              {adminSection === 'Seller Verification' && (
                <div className="dashboard-card admin-panel">
                  <div className="panel-header"><h3>Seller verification</h3><span>{adminPendingSellers.length} pending</span></div>
                  <div className="admin-record-list">
                    {adminPendingSellers.length === 0 ? <p className="auth-message">No sellers are waiting for verification.</p> : adminPendingSellers.map((seller) => <div className="admin-record-row" key={seller.id}>
                      <div><strong>{seller.name}</strong><span>{seller.email} · {seller.phone || 'No phone'}</span></div>
                      <span>{villages.find((village) => village.id === seller.villageId)?.name || 'Village not listed'}</span>
                      <div className="admin-row-actions"><button type="button" className="primary-button small" onClick={() => handleAdminSellerAction(seller.id, 'verify')}>Verify</button><button type="button" className="danger-button" onClick={() => handleAdminSellerAction(seller.id, 'reject')}>Reject</button></div>
                    </div>)}
                  </div>
                </div>
              )}

              {adminSection === 'Analytics' && (
                <div className="admin-analytics-view">
                  <div className="dashboard-grid admin-stats-grid">
                    <div className="stat-card"><span>Total users</span><strong>{adminStats.users || adminUsers.length}</strong></div>
                    <div className="stat-card"><span>Active sellers</span><strong>{adminSellers.filter((seller) => seller.active !== false && seller.verified).length}</strong></div>
                    <div className="stat-card"><span>Active agents</span><strong>{adminAgents.filter((agent) => agent.active !== false).length}</strong></div>
                    <div className="stat-card"><span>Delivered orders</span><strong>{adminStats.deliveredOrders || 0}</strong></div>
                    <div className="stat-card"><span>Failed deliveries</span><strong>{adminDeliveries.filter((delivery) => delivery.status === 'failed').length}</strong></div>
                    <div className="stat-card"><span>Revenue</span><strong>{formatCurrency(Number(adminStats.revenue || 0))}</strong></div>
                  </div>
                  <div className="admin-chart-grid">
                    {renderAdminBars('Sales by village', adminStats.salesByVillage || [], 'name', 'revenue')}
                    {renderAdminBars('Sales by category', adminStats.salesByCategory || [], 'category', 'revenue')}
                    {renderAdminBars('Daily sales', (adminStats.salesByDay || []).slice(-10), 'date', 'revenue')}
                    <div className="dashboard-card admin-chart-card">
                      <h3>Top-selling products</h3>
                      {(adminStats.topSellingProducts || []).length === 0 ? <p className="auth-message">No delivered product sales yet.</p> : <div className="admin-bar-list">{adminStats.topSellingProducts.map((product) => <div className="admin-bar-row" key={product.productId}><div><span>{product.name}</span><strong>{product.unitsSold} units</strong></div><small>{formatCurrency(Number(product.revenue || 0))}</small></div>)}</div>}
                    </div>
                    <div className="dashboard-card admin-chart-card">
                      <h3>Low-stock products</h3>
                      {(adminStats.lowStockProducts || []).length === 0 ? <p className="auth-message">No products at or below 5 units.</p> : <div className="admin-bar-list">{adminStats.lowStockProducts.map((product) => <div className="admin-bar-row" key={product.id}><div><span>{product.name}</span><strong>{product.stock} in stock</strong></div><small>{product.village}</small></div>)}</div>}
                    </div>
                  </div>
                </div>
              )}
            </>
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
                {authMode === 'register' ? 'Password (8-72 characters)' : 'Password'}
                <input
                  name="password"
                  type="password"
                  value={authForm.password}
                  onChange={handleAuthChange}
                  placeholder="Password"
                  autoComplete={authMode === 'register' ? 'new-password' : 'current-password'}
                  minLength={authMode === 'register' ? 8 : undefined}
                  maxLength={72}
                  required
                />
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

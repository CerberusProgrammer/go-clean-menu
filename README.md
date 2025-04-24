# Go-Clean-Menu 🍽️

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13%2B-336791?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![HTMX](https://img.shields.io/badge/HTMX-Powered-3366BB?style=for-the-badge)](https://htmx.org/)
[![Bootstrap](https://img.shields.io/badge/Bootstrap-5.3-7952B3?style=for-the-badge&logo=bootstrap&logoColor=white)](https://getbootstrap.com/)

A modern, elegant, and easy-to-use restaurant management system built with Go. Go-Clean-Menu helps you manage orders, kitchen operations, menus, and more with a clean, intuitive interface.

## 🌟 Features

- **📝 Order Management**: Create, edit, and track orders in real-time
- **👨‍🍳 Kitchen Display System**: Streamline kitchen operations with a dedicated view
- **🧾 Menu Administration**: Easily manage your products and categories
- **🔄 Real-time Updates**: WebSocket-based updates across the application
- **📊 Sales Analytics**: Track sales, popular products, and kitchen performance
- **🗓️ Order History**: Complete historical record of all orders
- **🛎️ Table Management**: Manage restaurant tables and their status
- **🔧 System Configuration**: Customize settings to match your restaurant needs
- **💾 Data Backup**: Create and download database backups
- **🌓 Light/Dark Mode**: Toggle between light and dark themes
- **🔐 Self-hosted**: Full control over your data and deployment
- **📱 Responsive Design**: Works on desktop and mobile devices

## 📸 Screenshots

### Dashboard
![Dashboard](https://github.com/user-attachments/assets/f835e3b4-cada-4453-97f4-a908151881ca)

### Orders Management
![Orders Management](https://github.com/user-attachments/assets/bbf07d33-c3d7-41b1-88dd-5e2340a13df7)

## 🚀 Getting Started

### Quick Start with Docker Compose

1. Clone the repository:
   ```bash
   git clone https://github.com/CerberusProgrammer/go-clean-menu.git
   cd go-clean-menu
   ```

2. Start with Docker Compose:
   ```bash
   docker-compose up -d
   ```

3. Access the application:
   ```
   http://localhost:3001
   ```

### Manual Installation

#### Prerequisites:
- Go 1.24+
- PostgreSQL 13+

#### Steps:
1. Clone the repository and navigate to the project directory
2. Configure the database in `.env` file
3. Install dependencies: `go mod download`
4. Build the application: `go build -o app`
5. Run the application: `./app`

## 🧠 Architecture

Go-Clean-Menu follows clean architecture principles with a focus on maintainability and testability:

```
go-clean-menu/
├── handlers.go      # HTTP request handlers
├── helpers.go       # Utility functions
├── history.go       # Order history functionality
├── kitchen.go       # Kitchen display system
├── main.go          # Application entry point
├── menu.go          # Menu management
├── models.go        # Data models
├── orders.go        # Order processing logic
├── settings.go      # Application settings
├── tables.go        # Table management
├── templates/       # HTML templates (using Go templates)
│   ├── layouts/     # Layout templates
│   └── partials/    # Reusable components
├── Dockerfile       # Docker configuration
└── docker-compose.yml  # Docker Compose configuration
```

## 🛠️ Tech Stack

- **Backend**: Go with Fiber framework
- **Database**: PostgreSQL
- **Frontend**: HTML, Bootstrap 5, HTMX
- **Real-time Updates**: WebSockets
- **Containerization**: Docker
- **Design Pattern**: MVC architecture
- **Template Engine**: Go HTML Templates

## 💻 Development

### Environment Variables

Configure the application by modifying the `.env` file:

```
DB_HOST=db
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=go_clean_menu
PORT=3001
```

### Project Structure

- **Templates**: HTML files in the `templates` directory
- **Static Assets**: CSS, JS, and images in the `static` directory
- **Logic**: Go files in the root directory, separated by functionality
- **Database**: PostgreSQL with models defined in `models.go`

### Key Components

- **Order Lifecycle**: pending → in_progress → ready → to_pay → completed
- **Kitchen Display**: Real-time view of pending orders for kitchen staff
- **Analytics**: Sales tracking and kitchen performance metrics
- **Product Management**: Categories and products with availability status

## 🤝 Contributing

Contributions are welcome! Here's how you can help:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure your code follows the project's coding style and includes appropriate tests.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- [Fiber](https://github.com/gofiber/fiber) - Fast HTTP framework for Go
- [HTMX](https://htmx.org/) - High-power tools for HTML
- [Bootstrap](https://getbootstrap.com/) - Frontend toolkit
- [Inter Font](https://fonts.google.com/specimen/Inter) - Clean, modern typography

---

Made with ❤️ by [CerberusProgrammer](https://github.com/CerberusProgrammer)

[⬆ Back to top](#go-clean-menu-)
